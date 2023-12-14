// Package worker is the main application for the nlp-worker service. It is triggered by a storage bucket Finalize event. It submits a file for NLP processing.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/storage"
	"github.com/rs/zerolog/log"

	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/nlp-worker/libs/utils"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	functions.CloudEvent("Handler", handler)
}

// handler is the cloud function entrypoint
func handler(ctx context.Context, e event.Event) error {
	if e.Type() != "google.cloud.storage.object.v1.finalized" {
		return fmt.Errorf("unsupported event type: %s", e.Type())
	}

	// app config
	// cfg := getConfig()

	// unmarshal event data
	var data storagedata.StorageObjectData
	if err := protojson.Unmarshal(e.Data(), &data); err != nil {
		return fmt.Errorf("protojson.Unmarshal: %w", err)
	}

	// bucket
	b := data.GetBucket()

	// filename
	f := data.GetName()

	// create storage client
	store, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to create storage client")
		return err
	}
	defer store.Close()

	// get object handle
	reader, err := store.Bucket(b).Object(f).NewReader(ctx)
	if err != nil {
		log.Fatal().Err(err).Caller().Str("bucket", b).Str("file", f).Msg("failed to create object reader")
		return err
	}
	defer reader.Close()

	// Read the entire object into a byte slice.
	jso, err := io.ReadAll(reader)
	if err != nil {
		log.Fatal().Err(err).Caller().Str("bucket", b).Str("file", f).Msg("failed to create object reader")
		return err
	}

	// unmarshal json to gcp ocr output type
	var doc documentaipb.Document
	err = json.Unmarshal(jso, &doc)
	if err != nil {
		log.Fatal().Err(err).Caller().Str("bucket", b).Str("file", f).Msg("failed to parse document JSON")
		return err
	}

	// get text from ocr output
	text := doc.GetText()

	fmt.Println(text)

	// filter text (watermark)

	// submit to nlp

	// wait response

	// write response to nlp-data bucket, include text

	// write err to err-bucket

	return nil
}

type appConfig struct {
	Debug          bool
	Port           string
	ProjectID      string
	DstBucketName  string
	ErrBucketName  string
	RefsBucketName string
}

func getConfig() appConfig {
	debug := utils.GetBoolEnvVar("DEBUG", false)
	port := utils.GetStrEnvVar("PORT", "8082")

	// gcp
	projectID := getMandatoryEnvVar("GCP_PROJECT_ID")

	// buckets
	dstBucketName := getMandatoryEnvVar("DST_BUCKET_NAME")
	errBucketName := getMandatoryEnvVar("ERR_BUCKET_NAME")
	refsBucketName := getMandatoryEnvVar("REFS_BUCKET_NAME")

	return appConfig{
		Debug:          debug,
		Port:           port,
		ProjectID:      projectID,
		DstBucketName:  dstBucketName,
		RefsBucketName: refsBucketName,
		ErrBucketName:  errBucketName,
	}
}

func getMandatoryEnvVar(n string) string {
	v := os.Getenv(n)
	if v != "" {
		return v
	}
	log.Fatal().Err(errors.New("missing env var")).Caller().Msgf("env var %s required", n)
	return ""
}
