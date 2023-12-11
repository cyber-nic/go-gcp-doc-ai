package deduper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"image"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	// Import image format packages

	_ "image/jpeg"
	_ "image/png"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// imageDocument represents the computed hash and its associated image paths, along with additional metadata.
type imageDocument struct {
	Hash       string   `firestore:"hash"`
	MimeType   string   `firestore:"mime_type"`
	ImagePaths []string `firestore:"image_paths"`
	Width      int      `firestore:"width"`
	Height     int      `firestore:"height"`
	Pixels     int      `firestore:"pixels"`
	Size       int64    `firestore:"size"`
}

type fileDocument struct {
	Hash string `firestore:"hash"`
}

func init() {
	// Register HTTP function with the Functions Framework
	functions.HTTP("Dedup", deduper)
}

// Function Dispatcher is an HTTP handler
func deduper(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// input
	projectID := getMandatoryEnvVar("GCP_PROJECT_ID")
	fireDatabaseID := getMandatoryEnvVar("FIRESTORE_DATABASE_ID")
	fireImageCollectionName := getMandatoryEnvVar("FIRESTORE_IMAGE_COLLECTION_NAME")
	fireFileCollectionName := getMandatoryEnvVar("FIRESTORE_FILE_COLLECTION_NAME")
	bucketName := getMandatoryEnvVar("BUCKET_NAME")
	checkpointBucketName := getMandatoryEnvVar("BUCKET_CHECKPOINT_NAME")
	// prefix is the prefix of the files to be processed. It allows for running
	// smaller more targeted batches
	bucketPrefix := GetStrEnvVar("BUCKET_PREFIX", "**/*.jpg")
	// maxFiles is the total number of images the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxFiles := GetIntEnvVar("MAX_FILES", 0)
	progressCount := GetIntEnvVar("PROGRESS_COUNT", 1000)

	// Initialize Firestore client.
	fire, err := firestore.NewClientWithDatabase(ctx, projectID, fireDatabaseID)
	if err != nil {
		log.Fatalf("failed to create Firestore client: %v", err)
	}
	defer fire.Close()

	images := fire.Collection(fireImageCollectionName)
	files := fire.Collection(fireFileCollectionName)

	// Create storage client.
	store, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create storage client: %v", err)
	}
	defer store.Close()

	// hasher is used to compute image hash.
	hasher := sha256.New()

	// checkpoint
	checkpointObj := store.Bucket(checkpointBucketName).Object("checkpoint")
	// read value
	checkpoint := getCheckpoint(ctx, checkpointObj)
	if checkpoint != "" {
		log.Printf("(checkpoint) %s\n", checkpoint)
	}

	// Iterate through all objects in the bucket.
	bucket := store.Bucket(bucketName)
	itr := bucket.Objects(ctx, &storage.Query{
		MatchGlob: bucketPrefix,
	})

	// track files and batches counts
	fileIdx := 0
	checkpointReached := false

	for {
		attrs, err := itr.Next()
		if err == iterator.Done {
			log.Println("iterator done")
			break
		}
		if err != nil {
			log.Fatalf("failed to iterate bucket objects: %v", err)
		}

		// itr control
		fileIdx++
		if maxFiles > 0 && fileIdx >= maxFiles {
			log.Printf("MAX FILES REACHED: %d of %d\n", fileIdx, maxFiles)
			break
		}

		// if checkpoint, skip until checkpoint
		if !checkpointReached && checkpoint != "" && checkpoint != attrs.Name {
			continue
		}
		checkpointReached = true

		// performed every `progressCount` files (ie. ~1,000)
		if fileIdx % progressCount == 0 {
			log.Printf("%d files processed\n", fileIdx)

			// update checkpoint every `progressCount`
			if checkpoint != attrs.Name {
				log.Printf("(checkpoint) current: %s, next: %s\n", checkpoint, attrs.Name)
				// open writer
				w := checkpointObj.NewWriter(ctx)
				defer w.Close()
				// update checkpoint
				if _, err := w.Write([]byte(attrs.Name)); err != nil {
					log.Printf("failed to write checkpoint (%s): %v\n", attrs.Name, err)
				}
				// close writer
				if err := w.Close(); err != nil {
					log.Printf("failed to close checkpoint writer (%s): %v\n", attrs.Name, err)
				}
			}

		}

		// process
		err = processFile(ctx, hasher, images, files, bucket, attrs)
		if err != nil && status.Code(err) == codes.PermissionDenied {
			log.Fatalf("%v", err)
		}

		// reset hasher
		hasher.Reset()
	}

	log.Println("done")
	return
}

func getMimeTypeFromExt(name string) (string, error) {
	var m string
	// mime type
	ext := filepath.Ext(name)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return m, fmt.Errorf("Failed to get mime type for %s", name)
	}

	return m, nil

}

func processFile(
	ctx context.Context,
	hasher hash.Hash,
	images *firestore.CollectionRef,
	files *firestore.CollectionRef,
	bucket *storage.BucketHandle,
	attrs *storage.ObjectAttrs,
) error {

	// get object handle
	obj := bucket.Object(attrs.Name)

	// filename
	filename := GetFilenameFromPath(attrs.Name)

	// Check if file exists in Firestore.
	fileRef := files.Doc(filename)
	_, err := fileRef.Get(ctx)
	if err == nil {
		// log.Printf("Skip %s", attrs.Name)
		return nil
	}
	if err != nil && status.Code(err) != codes.NotFound {
		log.Printf("failed to get document (%s): %s %v", filename, status.Code(err), err)
		return err
	}

	// skip a few empty files
	if attrs.Size == 0 {
		// log.Printf("Skip empty %s", attrs.Name)
		return nil
	}

	// Compute image hash.
	// log.Printf("Process %s", attrs.Name)

	// mime type
	mimeType, err := getMimeTypeFromExt(attrs.Name)
	if err != nil {
		log.Print(err)
		return err
	}

	// Creates a Reader to enable reading te object contents.
	reader, err := obj.NewReader(ctx)
	if err != nil {
		log.Printf("Failed to download object: %v", err)
		return err
	}
	defer reader.Close()

	// Download the object content to a buffer.
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		log.Printf("Failed to read image content: %v", err)
		return err
	}
	// if used directly, the buffer pointer will be at the end of the buffer at the end of the read.
	// As a result a practical solution is to create a new bytes reader for each use
	// bytes.NewReader(buf.Bytes())

	// clean up file reader
	reader.Close()

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		// todo: Printf
		log.Fatalf("Failed to decode image: %v", err)
		return err
	}

	// Get image dimensions and pixel count.
	width := img.Bounds().Max.X
	height := img.Bounds().Max.Y
	pixels := width * height

	// get hash
	hash := computeHash(hasher, bytes.NewReader(buf.Bytes()))

	// log.Println("hash", hash, "width", width, "height", height, "pixels", pixels)

	// Check if hash exists in Firestore.
	imgRef := images.Doc(hash)
	imgSnap, err := imgRef.Get(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		log.Printf("failed to get processed document: %v", err)
		return err
	}

	// Create or update document with image path.
	if !imgSnap.Exists() {
		_, err = imgRef.Set(ctx, &imageDocument{
			Hash:       hash,
			MimeType:   mimeType,
			Width:      width,
			Height:     height,
			Pixels:     pixels,
			Size:       attrs.Size,
			ImagePaths: []string{attrs.Name},
		})
		if err != nil {
			log.Printf("failed to set fire doc: %v", err)
			return err
		}
	} else {
		imageDoc := &imageDocument{}
		err = imgSnap.DataTo(imageDoc)
		if err != nil {
			log.Printf("failed to decode fire doc: %v", err)
			return err
			// break
		}
		if !slices.Contains(imageDoc.ImagePaths, attrs.Name) {
			imageDoc.ImagePaths = append(imageDoc.ImagePaths, attrs.Name)
			_, err = imgRef.Set(ctx, imageDoc)
			if err != nil {
				log.Printf("failed to set fire doc: %v", err)
				return err
			}
		}
	}

	// create file ref
	if _, err = fileRef.Set(ctx, &fileDocument{
		hash,
	}); err != nil {
		log.Printf("failed to create file ref: %v", err)
		return err
	}

	return nil
}

// getCheckpoint reads the checkpoint object from the checkpoint bucket.
func getCheckpoint(ctx context.Context, o *storage.ObjectHandle) string {
	// create checkpoint file if it is missing
	_, err := o.NewReader(ctx)
	// create file is missing
	if err != nil && err == storage.ErrObjectNotExist {
		w := o.NewWriter(ctx)
		defer w.Close()
		if _, err := w.Write([]byte("")); err != nil {
			log.Printf("failed to write checkpoint: %v\n", err)
		} else {
			log.Printf("checkpoint file created\n")
		}
	// Close the writer.
	if err := w.Close(); err != nil {
		log.Fatalf("failed to close object writer: %v", err)
	}
	} else if err != nil {
		// fail is unexpected error
		log.Print(err)
		log.Fatalf("failed to create checkpoint reader: %v", err)
	}

	r, err := o.NewReader(ctx)
	if err != nil {
		// fail is unexpected error
		log.Print(err)
		log.Fatalf("failed to create checkpoint reader: %v", err)
	}

	// var w
	var b []byte

	// Read the entire object into a byte slice.
	b, err = io.ReadAll(r)
	if err != nil {
		log.Fatalf("failed to read object: %v", err)
	}

	// Close the reader.
	if err := r.Close(); err != nil {
		log.Fatalf("failed to close object reader: %v", err)
	}

	return string(b)
}

// checkProcessed checks if a given image has already been processed.
func checkProcessed(client *firestore.Client, imageName string) (bool, error) {
	// Create the document reference for the processed image flag.
	docRef := client.Collection("processed_images").Doc(imageName)

	// Get the document snapshot.
	docSnap, err := docRef.Get(context.Background())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// Image not found, considered not processed.
			return false, nil
		}
		return false, fmt.Errorf("failed to get processed document: %w", err)
	}

	// Check the processed flag value.
	var processed bool
	err = docSnap.DataTo(&processed)
	if err != nil {
		return false, fmt.Errorf("failed to decode processed flag: %w", err)
	}

	return processed, nil
}

func computeHash(hasher hash.Hash, r *bytes.Reader) string {
	_, err := io.Copy(hasher, r)
	if err != nil {
		log.Printf("Failed to compute hash: %v", err)
		return ""
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

func getMandatoryEnvVar(n string) string {
	v := os.Getenv(n)
	if v != "" {
		return v
	}
	log.Fatalf("%s required", n)
	return ""
}

// GetIntEnvVar returns an int from an environment variable
func GetIntEnvVar(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err != nil {
			log.Fatal("Invalid value for environment variable: " + key)
		}
		return i
	}
	return fallback
}

// GetStrEnvVar returns a string from an environment variable
func GetStrEnvVar(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// GetBoolEnvVar returns a bool from an environment variable
func GetBoolEnvVar(key string, fallback bool) bool {
	val := GetStrEnvVar(key, strconv.FormatBool(fallback))
	ret, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return ret
}

func GetFilenameFromPath(f string) string {
	// Split the object name into parts
	parts := strings.Split(f, "/")

	// Extract the filename
	filename := parts[len(parts)-1]

	return filename
}
