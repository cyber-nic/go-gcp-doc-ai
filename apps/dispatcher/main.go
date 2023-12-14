// Package main is the main application for the dispatcher service. It reads from a firestore collection and publishes batches of filenames to a pubsub topic.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/dispatcher/libs/types"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/dispatcher/libs/utils"
)

// Dispatcher is an HTTP handler
func main() {
	// context
	ctx := context.Background()

	// app config
	cfg := getConfig()

	// create storage client
	s, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to create storage client")
	}
	defer s.Close()

	// ref bucket
	refsBucket := s.Bucket(cfg.RefsBucketName)
	if _, err := refsBucket.Attrs(ctx); err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to get refs bucket")
	}

	// Initialize Firestore client.
	db, err := firestore.NewClientWithDatabase(ctx, cfg.ProjectID, cfg.FireDatabaseID)
	if err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to create Firestore client")
	}
	defer db.Close()

	// create pubsub client and topic handler
	ps, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to create Pub/Sub client")
	}
	topic := ps.Topic(cfg.PubsubTopicID)
	defer topic.Stop()

	// checkpoint
	checkpointBucket := s.Bucket(cfg.CheckpointBucketName)
	if _, err := checkpointBucket.Attrs(ctx); err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to get checkpoint bucket")
	}
	checkpointObj := checkpointBucket.Object("checkpoint")
	checkpoint := utils.GetValueFromBucketFile(ctx, checkpointObj)
	log.Info().
		Str("checkpoint", shortStr(checkpoint, 12)).
		Msgf("initial checkpoint: %s", func() string {
			if checkpoint != "" {
				return shortStr(checkpoint, 12)
			} else {
				return "(none)"
			}
		}())

	// build query
	query := db.Collection(cfg.FireCollectionName).OrderBy("hash", firestore.Asc).StartAfter(checkpoint).Limit(cfg.BatchSize)

	// track files and batches counts
	fileIdx := 0
	batchedFilesCnt := 0
	batchIdx := 0

	// init doc batch
	docs := []string{}
	imgIDs := []string{}

	// Iterate through all objects in the firestore collection
Batch:
	for {
		snaps, err := query.Documents(ctx).GetAll()
		if err != nil {
			break Batch
		}
		if len(snaps) == 0 {
			break Batch // No more documents
		}

		// new checkpoint var
		newCheckpoint := ""

		// process batch
	Snap:
		for _, snap := range snaps {
			fileIdx++

			// Check if file was already processed
			if ok, err := existsInRefsBucket(ctx, refsBucket, snap.Ref.ID); err != nil || ok {
				// todo: if err write to src-err
				continue Snap
			}

			// Marshal the map to a JSON byte slice
			jsonBytes, err := json.Marshal(snap.Data())
			if err != nil {
				log.Fatal().Err(err).Caller().Msg("failed to marshal firestore document")
			}

			// Unmarshal the JSON data into the struct
			var imgdoc types.ImageDocument
			err = json.Unmarshal(jsonBytes, &imgdoc)
			if err != nil {
				log.Fatal().Err(err).Caller().Msg("failed to unmarshal firestore document")
			}

			// add file to batch
			docs = append(docs, fmt.Sprintf("gs://%s/%s", cfg.SrcBucketName, imgdoc.ImagePaths[0]))
			imgIDs = append(imgIDs, snap.Ref.ID)
			newCheckpoint = snap.Ref.ID
		}

		// in the odd event all docs returned from firestore were already processed
		if len(docs) == 0 {
			continue Batch
		}

		if cfg.Debug {
			for i := range docs {
				fmt.Printf("%d  %s     %s\n", i, imgIDs[i], docs[i])
			}
		}

		// send batch
		_, err = publishFilenameBatch(ctx, topic, docs)
		if err != nil {
			log.Error().Err(err).Caller().Msg("failed to publish pubsub batch")
			continue Batch
		}

		// inc batch count and log
		batchIdx++
		batchedFilesCnt += len(docs)
		log.Info().
			Int("files processed", fileIdx).
			Int("files sent", batchedFilesCnt).
			Int("batch id", batchIdx).
			Int("files in latest batch", len(docs)).
			Msgf("batch %d published (%d files)", batchIdx, batchedFilesCnt)

		// write refs to refs bucket
		if errs := writeRefs(ctx, refsBucket, imgIDs); len(errs) > 0 {
			log.Error().Err(err).Caller().Msg("failed to write refs")
		}

		// update checkpoint
		if checkpoint != newCheckpoint {
			log.Info().
				Int("files", fileIdx).
				Str("checkpoint", shortStr(checkpoint, 12)).
				Str("next", shortStr(newCheckpoint, 12)).
				Msgf("%d files processed, next checkpoint: %s", fileIdx, shortStr(newCheckpoint, 12))
			utils.SetBucketFileValue(ctx, checkpointObj, newCheckpoint)
			checkpoint = newCheckpoint
		}

		// reset docs
		docs = []string{}
		imgIDs = []string{}

		// Limit file count
		if cfg.MaxFiles > 0 && batchedFilesCnt >= cfg.MaxFiles {
			log.Info().
				Int("files", fileIdx).
				Int("max", cfg.MaxFiles).
				Int("sent files", batchedFilesCnt).
				Msg("MAX FILES REACHED")
			// exit the Batch loop
			break Batch
		}

		// Limit batch count
		if cfg.MaxBatch > 0 && batchIdx >= cfg.MaxBatch {
			log.Info().Int("files", fileIdx).Int("batch", batchIdx).Msg("MAX BATCH REACHED")
			break Batch
		}
	}

	// Send any remaining files in a final batch
	if len(docs) > 0 {
		// Send batch
		_, err = publishFilenameBatch(ctx, topic, docs)
		if err != nil {
			log.Error().Err(err).Caller().Msg("failed to publish pubsub batch")
		} else {
			// write refs to refs bucket
			if errs := writeRefs(ctx, refsBucket, imgIDs); len(errs) > 0 {
				for _, err := range errs {
					log.Error().Err(err).Caller().Msg("failed to write ref")
				}
			}
		}
	}

	log.Info().
		Int("files processed", fileIdx).
		Int("files sent", batchedFilesCnt).
		Int("batch count", batchIdx).
		Msg("done")
}

func shortStr(s string, i int) string {
	if len(s) > i {
		return s[:i]
	}
	return s
}

func writeRefs(ctx context.Context, bucket *storage.BucketHandle, docs []string) []error {
	var errs []error
	for _, d := range docs {
		if _, err := writeRef(ctx, bucket, utils.GetFilenameFromPath(d), d); err != nil {
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
		log.Fatal().Err(err).Caller().Msg("failed to check refs bucket")
	}

	return true, nil
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
	log.Fatal().Err(errors.New("missing env var")).Caller().Msgf("env var %s required", n)
	return ""
}

type appConfig struct {
	Debug                bool
	ProjectID            string
	FireDatabaseID       string
	FireCollectionName   string
	SrcBucketName        string
	RefsBucketName       string
	CheckpointBucketName string
	BatchSize            int
	MaxFiles             int
	MaxBatch             int
	PubsubTopicID        string
}

func getConfig() appConfig {
	debug := utils.GetBoolEnvVar("DEBUG", false)

	// gcp
	projectID := getMandatoryEnvVar("GCP_PROJECT_ID")

	// bucket
	srcBucketName := getMandatoryEnvVar("SRC_BUCKET_NAME")
	refsBucketName := getMandatoryEnvVar("REFS_BUCKET_NAME")
	checkpointBucketName := getMandatoryEnvVar("CHECKPOINT_BUCKET_NAME")

	// firestore
	fireDatabaseID := getMandatoryEnvVar("FIRESTORE_DATABASE_ID")
	fireCollectionName := getMandatoryEnvVar("FIRESTORE_COLLECTION_NAME")

	// pubsub
	pubsubTopicID := getMandatoryEnvVar("PUBSUB_TOPIC_ID")

	// limits
	batchSize := utils.GetIntEnvVar("BATCH_SIZE", 100)
	// maxFiles is the total number of images the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxFiles := utils.GetIntEnvVar("MAX_FILES", 0)
	// maxBatch is the total number of batches the system will process
	// before terminating. Mainly used for testing/sampling. Zero means no limit.
	maxBatch := utils.GetIntEnvVar("MAX_BATCH", 0)

	return appConfig{
		Debug:                debug,
		ProjectID:            projectID,
		FireDatabaseID:       fireDatabaseID,
		FireCollectionName:   fireCollectionName,
		SrcBucketName:        srcBucketName,
		RefsBucketName:       refsBucketName,
		CheckpointBucketName: checkpointBucketName,
		BatchSize:            batchSize,
		MaxFiles:             maxFiles,
		MaxBatch:             maxBatch,
		PubsubTopicID:        pubsubTopicID,
	}
}
