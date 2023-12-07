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

// Function Dispatcher is an HTTP handler
func Dispatcher(w http.ResponseWriter, r *http.Request) {
    // context
	ctx := context.Background()

    // inputs and configs
    gcpProjectID := os.Getenv("GCP_PROJECT_ID")
    if gcpProjectID == "" {
        log.Fatalf("GCP_PROJECT_ID required")
        return
    }
    srcBucketName := os.Getenv("SRC_BUCKET_NAME")
    refsBucketName := os.Getenv("REFS_BUCKET_NAME")
    if srcBucketName == "" || refsBucketName == "" {
        log.Fatalf("SRC_BUCKET_NAME and REFS_BUCKET_NAME required")
        return
    }
    ocrTopicName := os.Getenv("OCR_TOPIC_NAME")
    if ocrTopicName == "" {
        log.Fatalf("OCR_TOPIC_NAME required")
        return
    }
    
    // limits
    batchSize := utils.GetIntEnvVar("BATCH_SIZE", 100)
    // maxFiles is the total number of images the system will process
    // before terminating. Mainly used for testing/sampling. Zero means no limit.
    maxFiles := utils.GetIntEnvVar("MAX_FILES", 0)
    // maxBatch is the total number of batches the system will process
    // before terminating. Mainly used for testing/sampling. Zero means no limit.
    maxBatch := utils.GetIntEnvVar("MAX_BATCH", 0)



    // create storage client
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
		return
	}
	defer client.Close()

    // get bucket
	srcBucket := client.Bucket(srcBucketName)
	refsBucket := client.Bucket(refsBucketName)
	pubsubClient, err := pubsub.NewClient(ctx, gcpProjectID)
	if err != nil {
		log.Fatalf("Failed to create Pub/Sub client: %v", err)
		return
	}

	topic := pubsubClient.Topic(ocrTopicName)
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
