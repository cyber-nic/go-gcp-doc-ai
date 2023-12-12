// this application is used to identify duplicate images in a bucket by creating a firestore document for each unique image
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"image"
	"io"
	"log"
	"os"
	"slices"

	// Import image format packages

	_ "image/jpeg"
	_ "image/png"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/deduper/libs/types"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/deduper/libs/utils"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fileDocument struct {
	Hash string `firestore:"hash"`
}

func main() {
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
	bucketPrefix := utils.GetStrEnvVar("BUCKET_PREFIX", "**/*.jpg")
	// maxFiles is the total number of images the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxFiles := utils.GetIntEnvVar("MAX_FILES", 0)
	progressCount := utils.GetIntEnvVar("PROGRESS_COUNT", 1000)

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
	checkpointFilename := "checkpoint"
	checkpointObj := store.Bucket(checkpointBucketName).Object(checkpointFilename)
	// read value
	checkpoint := utils.GetValueFromBucketFile(ctx, checkpointObj)
	log.Printf("(checkpoint) %s\n", checkpoint)

	// Iterate through all objects in the bucket.
	bucket := store.Bucket(bucketName)
	itr := bucket.Objects(ctx, &storage.Query{
		MatchGlob: bucketPrefix,
	})

	// track files and batches counts
	fileIdx := 0
	skippedIdx := 0
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

		if !checkpointReached && fileIdx%progressCount == 0 {
			log.Printf("%d files processed (%d skipped)\n", fileIdx, skippedIdx)
		}

		// if checkpoint, skip until checkpoint
		if !checkpointReached && checkpoint != "" && checkpoint != attrs.Name {
			skippedIdx++
			continue
		}
		checkpointReached = true
		skippedIdx = 0

		// update checkpoint every `progressCount` files (ie. ~1,000)
		if fileIdx%progressCount == 0 && checkpoint != attrs.Name {
			log.Printf("%d files processed (%d skipped) : (checkpoint) next: %s\n", fileIdx, skippedIdx, attrs.Name)
			utils.SetBucketFileValue(ctx, checkpointObj, attrs.Name)
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
}

func processFile(
	ctx context.Context,
	hasher hash.Hash,
	images *firestore.CollectionRef,
	files *firestore.CollectionRef,
	bucket *storage.BucketHandle,
	attrs *storage.ObjectAttrs,
) error {

	// filename
	filename := utils.GetFilenameFromPath(attrs.Name)

	// Check if file exists in Firestore
	fileRef := files.Doc(filename)
	_, err := fileRef.Get(ctx)
	if err == nil {
		// log.Printf("Skip %s", attrs.Name)
		return nil
	}
	// fail if err but continue on NotFound
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
	mimeType, err := utils.GetMimeTypeFromExt(attrs.Name)
	if err != nil {
		log.Print(err)
		return err
	}

	// get object handle
	obj := bucket.Object(attrs.Name)

	// Creates a Reader to enable reading te object contents.
	reader, err := obj.NewReader(ctx)
	if err != nil {
		log.Printf("Failed to download object: %v (%s)", err, attrs.Name)
		return err
	}
	defer reader.Close()

	// Download the object content to a buffer.
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		log.Printf("Failed to read image content: %v (%s)", err, attrs.Name)
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
		log.Printf("failed to decode image: %v (%s)", err, attrs.Name)
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
		log.Printf("failed to get processed document: %v (%s)", err, attrs.Name)
		return err
	}

	// Create or update document with image path.
	if !imgSnap.Exists() {
		_, err = imgRef.Set(ctx, &types.ImageDocument{
			Hash:       hash,
			MimeType:   mimeType,
			Width:      width,
			Height:     height,
			Pixels:     pixels,
			Size:       attrs.Size,
			ImagePaths: []string{attrs.Name},
		})
		if err != nil {
			log.Printf("failed to set fire doc: %v (%s)", err, attrs.Name)
			return err
		}
	} else {
		imageDoc := &types.ImageDocument{}
		err = imgSnap.DataTo(imageDoc)
		if err != nil {
			log.Printf("failed to decode fire doc: %v (%s)", err, attrs.Name)
			return err
			// break
		}
		if !slices.Contains(imageDoc.ImagePaths, attrs.Name) {
			imageDoc.ImagePaths = append(imageDoc.ImagePaths, attrs.Name)
			_, err = imgRef.Set(ctx, imageDoc)
			if err != nil {
				log.Printf("failed to set fire doc: %v (%s)", err, attrs.Name)
				return err
			}
		}
	}

	// create file ref
	if _, err = fileRef.Set(ctx, &fileDocument{
		hash,
	}); err != nil {
		log.Printf("failed to create file ref: %v (%s)", err, attrs.Name)
		return err
	}

	return nil
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
