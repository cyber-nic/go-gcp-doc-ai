// Package worker is the main application for the nlp-worker service. It is triggered by a storage bucket Finalize event. It submits a file for NLP processing.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	language "cloud.google.com/go/language/apiv1"
	"cloud.google.com/go/language/apiv1/languagepb"
	"cloud.google.com/go/storage"
	"cloud.google.com/go/vision/v2/apiv1/visionpb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"google.golang.org/protobuf/encoding/protojson"
)

type appConfig struct {
	Debug         bool
	ProjectID     string
	DstBucketName string
	ErrBucketName string
}

var (
	cfg   appConfig
	nlp   *language.Client
	store *storage.Client
)

func init() {
	// app config
	cfg = getConfig()

	// clients
	var err error

	// create language client
	nlp, err = language.NewClient(context.Background())
	if err != nil {
		log.Fatal().Msgf("failed to create language client: %v", err)
	}

	// create storage client
	store, err = storage.NewClient(context.Background())
	if err != nil {
		log.Fatal().Msgf("failed to create storage client: %v", err)
	}

	// register handler
	functions.CloudEvent("Handler", handler)
}

// handler is the cloud function entrypoint
func handler(ctx context.Context, e event.Event) error {
	if e.Type() != "google.cloud.storage.object.v1.finalized" {
		return fmt.Errorf("unsupported event type: %s", e.Type())
	}

	// unmarshal event data
	var data storagedata.StorageObjectData
	if err := protojson.Unmarshal(e.Data(), &data); err != nil {
		return fmt.Errorf("protojson.Unmarshal: %w", err)
	}

	// err bucket
	errBucket := store.Bucket(cfg.ErrBucketName)

	// src bucket
	s := data.GetBucket()

	// src filename
	f := data.GetName()

	defer log.Info().Str("src_bucket", data.GetBucket()).Str("dst", cfg.DstBucketName).Msg(data.GetName())

	// get src object handle
	reader, err := store.Bucket(s).Object(f).NewReader(ctx)
	if err != nil {
		m := fmt.Sprintf("failed to create object reader (%s/%s)", s, f)
		writeErrorResponseToBucketFile(ctx, errBucket, f, m, err)
		return fmt.Errorf("%s: %w", m, err)
	}

	defer reader.Close()

	// read object into a byte slice
	jso, err := io.ReadAll(reader)
	if err != nil {
		m := fmt.Sprintf("failed to read file (%s/%s)", s, f)
		writeErrorResponseToBucketFile(ctx, errBucket, f, m, err)
		return fmt.Errorf("%s: %w", m, err)
	}

	// unmarshal protojson to gcp ocr output type
	var doc visionpb.BatchAnnotateImagesResponse
	err = protojson.Unmarshal(jso, &doc)
	if err != nil {
		m := fmt.Sprintf("failed to parse document JSON (%s/%s)", s, f)
		writeErrorResponseToBucketFile(ctx, errBucket, f, m, err)
		return fmt.Errorf("%s: %w", m, err)
	}

	for _, r := range doc.Responses {

		if r.Error != nil {
			m := fmt.Sprintf("failed to process ocr response (%s/%s)", s, f)
			writeErrorResponseToBucketFile(ctx, errBucket, f, m, fmt.Errorf("%s: %s", m, r.Error.Message))
			continue
		}

		// create nlp request
		req := &languagepb.AnalyzeEntitiesRequest{
			Document: &languagepb.Document{
				// https://pkg.go.dev/cloud.google.com/go/language/apiv1/languagepb#Document_Type
				Type: languagepb.Document_PLAIN_TEXT,
				Source: &languagepb.Document_Content{
					Content: r.FullTextAnnotation.Text,
				},
				// select most likely language from OCR output
				Language: r.FullTextAnnotation.Pages[0].Property.DetectedLanguages[0].LanguageCode,
			},
		}

		// perform nlp entities analysis
		resp, err := nlp.AnalyzeEntities(ctx, req)
		if err != nil {
			m := fmt.Sprintf("failed to analyze nlp entities (%s/%s)", s, f)
			writeErrorResponseToBucketFile(ctx, errBucket, f, m, err)
			return fmt.Errorf("%s: %w", m, err)
		}

		// write response to file
		wc := store.Bucket(cfg.DstBucketName).Object(f).NewWriter(ctx)
		wc.ContentType = "application/json"

		// marshal struct to JSON directly into the writer
		encoder := json.NewEncoder(wc)
		if err := encoder.Encode(resp); err != nil {
			m := fmt.Sprintf("failed to json encode nlp resp (%s/%s)", cfg.DstBucketName, f)
			writeErrorResponseToBucketFile(ctx, errBucket, f, m, err)
			return fmt.Errorf("%s: %w", m, err)
		}

		if err := wc.Close(); err != nil {
			m := fmt.Sprintf("failed to close json writer (%s/%s)", cfg.DstBucketName, f)
			writeErrorResponseToBucketFile(ctx, errBucket, f, m, err)
			return fmt.Errorf("%s: %w", m, err)
		}
	}

	return nil
}

func getConfig() appConfig {
	debug := GetBoolEnvVar("DEBUG", false)

	// gcp
	projectID := getMandatoryEnvVar("GCP_PROJECT_ID")

	// buckets
	dstBucketName := getMandatoryEnvVar("DST_BUCKET_NAME")
	errBucketName := getMandatoryEnvVar("ERR_BUCKET_NAME")

	return appConfig{
		Debug:         debug,
		ProjectID:     projectID,
		DstBucketName: dstBucketName,
		ErrBucketName: errBucketName,
	}
}

func getMandatoryEnvVar(n string) string {
	v, ok := os.LookupEnv(n)
	if !ok || v == "" {
		log.Fatal().Msgf("env var %s required", n)
	}
	return v
}

// writeErrorResponseToBucketFile writes a Go error response to a bucket file.
func writeErrorResponseToBucketFile(ctx context.Context, b *storage.BucketHandle, fileName, msg string, err error) error {
	// Create error response with timestamp and stack trace
	errorResponse := struct {
		Timestamp  time.Time `json:"timestamp"`
		Error      error     `json:"error"`
		StackTrace string    `json:"stack_trace,omitempty"`
		Message    string    `json:"message,omitempty"` // Optional custom data
	}{
		Timestamp:  time.Now(),
		Error:      err,
		StackTrace: fmt.Sprintf("%+v", err), // Capture stack trace
		Message:    msg,
	}

	wc := b.Object(fileName).NewWriter(ctx)
	defer func() {
		if cerr := wc.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	wc.ContentType = "application/json"

	encoder := json.NewEncoder(wc)
	if err := encoder.Encode(errorResponse); err != nil {
		return err
	}

	return nil
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
