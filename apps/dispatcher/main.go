package dispatcher

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

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
	Debug           bool
	ProjectID       string
	SrcBucketName   string
	SrcBucketPrefix string
	RefsBucketName  string
	OcrTopicName    string
	BatchSize       int
	MaxFiles        int
	MaxBatch        int
}

// Function Dispatcher is an HTTP handler
func Dispatcher(w http.ResponseWriter, r *http.Request) {
	// context
	ctx := context.Background()

	// app config
	cfg := getConfig()
	if cfg.Debug {
		utils.PrintStruct(cfg)
	}

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

	files := []*storage.ObjectAttrs{}

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

		// Check if file was already processed
		ok, err := existsInRefsBucket(ctx, refsBucket, toRef(attrs.CRC32C))
		if err != nil || ok {
			// todo: if err write to src-err
			continue
		}

		// Limit file count
		if cfg.MaxFiles > 0 && fileIdx > cfg.MaxFiles {
			log.Println("MAX FILES REACHED")
			break
		}
		fileIdx++

		// Batch files
		files = append(files, attrs)
		if len(files) >= cfg.BatchSize {
			// print batch
			log.Println("NEW BATCH")
			for _, a := range files {
				log.Println(a.Name)
			}
			// // Send batch
			// if err := sendBatch(ctx, topic, filenames); err != nil {
			// 	log.Printf("Failed to send batch: %v", err)
			// 	// Handle error
			// }

			// write refs to refs bucket
			for _, a := range files {
				if err := writeRef(ctx, refsBucket,  toRef(a.CRC32C), a.Name); err != nil {
					log.Printf("Failed to write ref: %v", err)
					// Handle error
				}
			}

			files = []*storage.ObjectAttrs{}
		}

		// Limit batch count
		if cfg.MaxBatch > 0 && batchIdx > cfg.MaxBatch {
			log.Println("MAX BATCH REACHED")
			break
		}
		batchIdx++
	}

	// // Send any remaining files in the final batch
	// if len(filenames) > 0 {
	// 	if err := sendBatch(ctx, topic, filenames); err != nil {
	// 		log.Printf("Failed to send final batch: %v", err)
	// 		// Handle error
	// 	}
	// }

	log.Println("done")
}

func writeRef(ctx context.Context, bucket *storage.BucketHandle, k string, v string) error {
	writer := bucket.Object(k).NewWriter(ctx)
	defer writer.Close()

	if _, err := writer.Write([]byte(v)); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

func toRef(crc32 uint32) string {
	return strconv.FormatUint(uint64(crc32), 10)
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

// func sendBatch(ctx context.Context, topic *pubsub.Topic, filenames []string) error {
// 	batch := Batch{Filenames: filenames}
// 	batchData, err := json.Marshal(batch)
// 	if err != nil {
// 		return err
// 	}
// 	result := topic.Publish(ctx, &pubsub.Message{
// 		Data: batchData,
// 	})
// 	_, err = result.Get(ctx)
// 	return err
// }

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
		OcrTopicName:    ocrTopicName,
		BatchSize:       batchSize,
		MaxFiles:        maxFiles,
		MaxBatch:        maxBatch,
	}
}
