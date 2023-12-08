package dispatcher

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

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

	filenames := []string{}

	q := &storage.Query{
		MatchGlob: cfg.SrcBucketPrefix,
	}

	// track files and batches counts
	fileIdx := 0
	batchIdx := 0

	itr := srcBucket.Objects(ctx, q)
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
		ok, err := existsInRefsBucket(ctx, refsBucket, getFilename(attrs.Name))
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

		// Batch files
		filenames = append(filenames, attrs.Name)
		if len(filenames) >= cfg.BatchSize {
			// inc batch count
			batchIdx++

			// Send batch
			enc, err := publishFilenameBatch(ctx, topic, filenames)
			if err != nil {
				log.Printf("Failed to publish pubsub batch: %v", err)
				// Handle error
				// is error for batch or single file?
				continue
			}
			log.Println("(batch)", "id:", batchIdx, "files:", len(filenames), "data", enc)

			// write refs to refs bucket
			if errs := writeRefs(ctx, refsBucket, filenames); errs != nil && len(errs) > 0 {
				log.Printf("Failed to write refs: %v", err)
			}

			filenames = []string{}
		}

		// Limit batch count
		if cfg.MaxBatch > 0 && batchIdx >= cfg.MaxBatch {
			log.Println("MAX BATCH REACHED")
			break
		}
	}

	// Send any remaining files in a final batch
	if len(filenames) > 0 {
		// write refs to refs bucket
		if errs := writeRefs(ctx, refsBucket, filenames); errs != nil && len(errs) > 0 {
			log.Printf("Failed to write refs: %v", err)
		}
	}

	if fileIdx == 0 && batchIdx == 0 {
		log.Println("(metris) none")
		return
	}
	log.Println("(metrics)", "batches:", batchIdx, "files:", fileIdx, )
}

func writeRefs(ctx context.Context, bucket *storage.BucketHandle, filenames []string) []error {
	var errs []error
	for _, n := range filenames {
		if _, err := writeRef(ctx, bucket, getFilename(n), n); err != nil {
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

func getFilename(f string) string {
	// Split the object name into parts
	parts := strings.Split(f, "/")

	// Extract the filename
	filename := parts[len(parts)-1]

	return filename
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

func publishFilenameBatch(ctx context.Context, t *pubsub.Topic, f []string) (string, error) {
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
