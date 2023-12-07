package dispatcher

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cyber-nic/go-gcp-docai-ocr/libs/utils"
)

func init() {
	// Register HTTP function with the Functions Framework
	functions.HTTP("dispatch", Dispatcher)
}

type appConfig struct {
	Debug          bool
	ProjectID      string
	SrcBucketName  string
	RefsBucketName string
	OcrTopicName   string
	BatchSize      int
	MaxFiles       int
	MaxBatch       int
}

// Function Dispatcher is an HTTP handler
func Dispatcher(w http.ResponseWriter, r *http.Request) {
	// context
	ctx := context.Background()

	// app config
	cfg := getConfig()

	// create storage client
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
	}
	defer client.Close()

	// create bucket handlers
	srcBucket := client.Bucket(cfg.SrcBucketName)
	refsBucket := client.Bucket(cfg.RefsBucketName)

	// create pubsub client and topic handler
	pubsubClient, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Fatalf("Failed to create Pub/Sub client: %v", err)
	}
	topic := pubsubClient.Topic(cfg.OcrTopicName)
	defer topic.Stop()

	var filenames []string
	it := srcBucket.Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err != nil {
			break
		}
		// Check if the file is already processed
		if existsInRefsBucket(ctx, refsBucket, attrs.Name) {
			continue
		}

		filenames = append(filenames, attrs.Name)
		if len(filenames) >= 100 {
			// Send batch
			if err := sendBatch(ctx, topic, filenames); err != nil {
				log.Printf("Failed to send batch: %v", err)
				// Handle error
			}
			filenames = []string{}
		}
	}

	// Send any remaining files in the final batch
	if len(filenames) > 0 {
		if err := sendBatch(ctx, topic, filenames); err != nil {
			log.Printf("Failed to send final batch: %v", err)
			// Handle error
		}
	}

}

func existsInRefsBucket(ctx context.Context, bucket *storage.BucketHandle, filename string) bool {
	// Implement logic to check if a file exists in the refs bucket
	// ...
	return false
}

func sendBatch(ctx context.Context, topic *pubsub.Topic, filenames []string) error {
	batch := Batch{Filenames: filenames}
	batchData, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	result := topic.Publish(ctx, &pubsub.Message{
		Data: batchData,
	})
	_, err = result.Get(ctx)
	return err
}


func getConfig() appConfig {
	debug := utils.GetBoolEnvVar("DEBUG", false)

	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		log.Fatalf("GCP_PROJECT_ID required")
	}
	srcBucketName := os.Getenv("SRC_BUCKET_NAME")
	refsBucketName := os.Getenv("REFS_BUCKET_NAME")
	if srcBucketName == "" || refsBucketName == "" {
		log.Fatalf("SRC_BUCKET_NAME and REFS_BUCKET_NAME required")
	}
	ocrTopicName := os.Getenv("OCR_TOPIC_NAME")
	if ocrTopicName == "" {
		log.Fatalf("OCR_TOPIC_NAME required")
	}

	// limits
	batchSize := utils.GetIntEnvVar("BATCH_SIZE", 100)
	// maxFiles is the total number of images the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxFiles := utils.GetIntEnvVar("MAX_FILES", 0)
	// maxBatch is the total number of batches the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxBatch := utils.GetIntEnvVar("MAX_BATCH", 0)

	return appConfig{
		Debug:          debug,
		ProjectID:      projectID,
		SrcBucketName:  srcBucketName,
		RefsBucketName: refsBucketName,
		OcrTopicName:   ocrTopicName,
		BatchSize:      batchSize,
		MaxFiles:       maxFiles,
		MaxBatch:       maxBatch,
	}
}
