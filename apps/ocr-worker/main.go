// Package worker is the main application for the nlp-worker service. It is triggered by a storage bucket Finalize event. It submits a file for OCR processing.
package worker

// https://cloud.google.com/vision/docs/handwriting#vision-document-text-detection-go

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"cloud.google.com/go/storage"
	vision "cloud.google.com/go/vision/v2/apiv1"
	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"google.golang.org/protobuf/encoding/protojson"
)

type appConfig struct {
	Debug         bool
	DstBucketName string
	ErrBucketName string
}

var (
	cfg   appConfig
	ocr   *vision.ImageAnnotatorClient
	store *storage.Client
)

func init() {
	var err error
	ctx := context.Background()

	// app config
	cfg = getConfig()

	// vision client
	ocr, err = vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		log.Fatal().Msgf("failed to create vision client: %v", err)
	}

	// create storage client
	store, err = storage.NewClient(ctx)
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

	errHandler := store.Bucket(cfg.ErrBucketName)

	// Check if the object is a file and not a directory/prefix
	if filepath.Ext(data.GetName()) == "" {
		return nil
	}

	// src filename
	path := strings.TrimPrefix(data.GetName(), "./")
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	filename := strings.TrimSuffix(base, filepath.Ext(base))

	srcURI := fmt.Sprintf("gs://%s/%s", data.GetBucket(), path)
	dstURI := fmt.Sprintf("gs://%s/%s/%s/", cfg.DstBucketName, dir, filename)
	defer log.Info().Str("src", srcURI).Str("dst", dstURI).Msg(path)

	req := &visionpb.AsyncBatchAnnotateImagesRequest{
		OutputConfig: &visionpb.OutputConfig{
			GcsDestination: &visionpb.GcsDestination{
				Uri: dstURI,
			},
		},
		// https://pkg.go.dev/cloud.google.com/go/vision/v2/apiv1/visionpb#AsyncBatchAnnotateImagesRequest
		Requests: []*visionpb.AnnotateImageRequest{
			{
				Image: &visionpb.Image{
					Source: &visionpb.ImageSource{
						ImageUri: srcURI,
					},
				},
				ImageContext: &visionpb.ImageContext{
					LanguageHints: []string{"ru", "uk", "pl", "lv", "lt", "et", "de", "es", "fr", "en", "ro", "be", "az", "he", "yi", "hy"},
					TextDetectionParams: &visionpb.TextDetectionParams{
						EnableTextDetectionConfidenceScore: true,
					},
				},
				Features: []*visionpb.Feature{
					{
						Type: visionpb.Feature_DOCUMENT_TEXT_DETECTION,
					},
				},
			},
		},
	}

	op, err := ocr.AsyncBatchAnnotateImages(ctx, req)
	if err != nil {
		writeErrorResponseToBucketFile(ctx, errHandler, path, "failed to create async batch annotation", err)
		log.Err(err).Msg("failed to create async batch annotation")
	}

	_, err = op.Wait(ctx)
	if err != nil {
		writeErrorResponseToBucketFile(ctx, errHandler, path, "failed to create async batch annotation", err)
		log.Err(err).Msg("batch annotation failed")
	}

	return nil
}

func getConfig() appConfig {
	return appConfig{
		Debug:         GetBoolEnvVar("DEBUG", false),
		DstBucketName: getMandatoryEnvVar("DST_BUCKET_NAME"),
		ErrBucketName: getMandatoryEnvVar("ERR_BUCKET_NAME"),
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
