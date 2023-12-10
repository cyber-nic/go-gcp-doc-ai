package dispatcher

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cyber-nic/go-gcp-docai-ocr/libs/utils"
)

func init() {
	// Register HTTP function with the Functions Framework
	functions.HTTP("dispatch", Dispatcher)
}

// Function Dispatcher is an HTTP handler
func Dispatcher(w http.ResponseWriter, r *http.Request) {
	// context
	ctx := context.Background()

	// app config
	cfg := getConfig()

	// create storage client
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
	}
	defer storageClient.Close()

	// create bucket handlers
	srcBucket := storageClient.Bucket(cfg.SrcBucketName)
	refsBucket := storageClient.Bucket(cfg.RefsBucketName)

	// create pubsub client and topic handler
	pubsubClient, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Fatalf("Failed to create Pub/Sub client: %v", err)
	}
	topic := pubsubClient.Topic(cfg.PubsubTopicID)
	defer topic.Stop()

	// filenames := []string{}
	docs := []documentaipb.GcsDocument{}

	// bucket iterator
	itr := srcBucket.Objects(ctx, &storage.Query{
		MatchGlob: cfg.SrcBucketPrefix,
	})

	// track files and batches counts
	fileIdx := 0
	batchIdx := 0

	for {
		attrs, err := itr.Next()
		if err != nil {
			break
		}

		// skip a few empty files
		if attrs.Size == 0 {
			continue
		}

		// Check if file was already processed
		ok, err := existsInRefsBucket(ctx, refsBucket, utils.GetFilenameFromPath(attrs.Name))
		if err != nil || ok {
			// todo: if err write to src-err
			continue
		}

		// Limit file count
		if cfg.MaxFiles > 0 && fileIdx >= cfg.MaxFiles {
			log.Println("MAX FILES REACHED")
			break
		}
		fileIdx++

		// mime type
		ext := filepath.Ext(attrs.Name)
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			log.Printf("Failed to get mime type for %s", attrs.Name)
			continue
		}

		// add file to batch
		docs = append(docs, documentaipb.GcsDocument{
			GcsUri:   fmt.Sprintf("gs://%s/%s", cfg.SrcBucketName, attrs.Name),
			MimeType: mimeType,
		})

		if len(docs) >= cfg.BatchSize {
			// inc batch count
			batchIdx++

			// Send batch
			enc, err := publishFilenameBatch(ctx, topic, docs)
			if err != nil {
				log.Printf("Failed to publish pubsub batch: %v", err)
				// Handle error
				// is error for batch or single file?
				continue
			}
			log.Println("(batch)", "id:", batchIdx, "files:", len(docs), "data", enc)

			// write refs to refs bucket
			if errs := writeRefs(ctx, refsBucket, docs); errs != nil && len(errs) > 0 {
				log.Printf("Failed to write refs: %v", err)
			}

			docs = []documentaipb.GcsDocument{}
		}

		// Limit batch count
		if cfg.MaxBatch > 0 && batchIdx >= cfg.MaxBatch {
			log.Println("MAX BATCH REACHED")
			break
		}
	}

	// Send any remaining files in a final batch
	if len(docs) > 0 {
		// write refs to refs bucket
		if errs := writeRefs(ctx, refsBucket, docs); errs != nil && len(errs) > 0 {
			log.Printf("Failed to write refs: %v", err)
		}
	}

	if fileIdx == 0 && batchIdx == 0 {
		log.Println("(metris) none")
		return
	}
	log.Println("(metrics)", "batches:", batchIdx, "files:", fileIdx)
}

func writeRefs(ctx context.Context, bucket *storage.BucketHandle, docs []documentaipb.GcsDocument) []error {
	var errs []error
	for _, d := range docs {
		if _, err := writeRef(ctx, bucket, utils.GetFilenameFromPath(d.GcsUri), d.GcsUri); err != nil {
			log.Printf("Failed to write ref: %v", err)
			// Handle individual errors
			errs = append(errs, err)
		}
	}
	return errs
}

func writeRef(ctx context.Context, bucket *storage.BucketHandle, k string, v string) (*storage.ObjectAttrs, error) {
	writer := bucket.Object(k).NewWriter(ctx)
	defer writer.Close()

	if _, err := writer.Write([]byte(v)); err != nil {
		return nil, err
	}

	return writer.Attrs(), nil
}

func existsInRefsBucket(ctx context.Context, bucket *storage.BucketHandle, filename string) (bool, error) {
	_, err := bucket.Object(filename).NewReader(ctx)
	if err != nil && err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

type MessageShema struct {
	Filenames string `json:"filenames"`
}

func publishFilenameBatch(ctx context.Context, t *pubsub.Topic, f []documentaipb.GcsDocument) (string, error) {
	var id string
	enc, err := utils.EncodeToBase64(f)
	if err != nil {
		return id, err
	}

	result := t.Publish(ctx, &pubsub.Message{
		Data: []byte(enc),
	})

	// Block until the result is returned and a server-generated
	// ID is returned for the published message.
	id, err = result.Get(ctx)
	if err != nil {
		return id, err
	}

	return id, nil
}

func getMandatoryEnvVar(n string) string {
	v := os.Getenv(n)
	if v != "" {
		return v
	}
	log.Fatalf("%s required", n)
	return ""
}

type appConfig struct {
	Debug           bool
	ProjectID       string
	SrcBucketName   string
	SrcBucketPrefix string
	RefsBucketName  string
	BatchSize       int
	MaxFiles        int
	MaxBatch        int
	PubsubProjectID string
	PubsubTopicID   string
}

func getConfig() appConfig {
	debug := utils.GetBoolEnvVar("DEBUG", false)

	// gcp
	projectID := getMandatoryEnvVar("GCP_PROJECT_ID")

	// buckets
	srcBucketName := getMandatoryEnvVar("SRC_BUCKET_NAME")
	refsBucketName := getMandatoryEnvVar("REFS_BUCKET_NAME")

	// pubsub
	pubsubProjectID := getMandatoryEnvVar("PUBSUB_PROJECT_ID")
	pubsubTopicID := getMandatoryEnvVar("PUBSUB_TOPIC_ID")

	// prefix is the prefix of the files to be processed. It allows for running
	// smaller more targeted batches
	srcBucketPrefix := utils.GetStrEnvVar("SRC_BUCKET_PREFIX", "**/*.jpg")

	// limits
	batchSize := utils.GetIntEnvVar("BATCH_SIZE", 100)
	// maxFiles is the total number of images the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxFiles := utils.GetIntEnvVar("MAX_FILES", 0)
	// maxBatch is the total number of batches the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxBatch := utils.GetIntEnvVar("MAX_BATCH", 0)

	return appConfig{
		Debug:           debug,
		ProjectID:       projectID,
		SrcBucketName:   srcBucketName,
		SrcBucketPrefix: srcBucketPrefix,
		RefsBucketName:  refsBucketName,
		BatchSize:       batchSize,
		MaxFiles:        maxFiles,
		MaxBatch:        maxBatch,
		PubsubProjectID: pubsubProjectID,
		PubsubTopicID:   pubsubTopicID,
	}
}
