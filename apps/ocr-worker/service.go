package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	documentai "cloud.google.com/go/documentai/apiv1"
	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/ocr-worker/libs/utils"
)

// SvcOptions is the representation of the options availble to the OCRWorkerSvc service
type SvcOptions struct {
	Topic            *pubsub.Topic
	Subscription     *pubsub.Subscription
	AIClient         *documentai.DocumentProcessorClient
	AIProcessorName  string
	DstBucketName    string
	RefsBucketHandle *storage.BucketHandle
}

// OCRWorkerSvc is the interface for the ocrWorkerSvc service.
type OCRWorkerSvc interface {
	IsReady() bool
	Start() error
	Stop()
}

// ocrWorkerSvc is a service that will submit a batch of documents to the Document AI API.
type ocrWorkerSvc struct {
	ready            bool
	Context          context.Context
	Topic            *pubsub.Topic
	Subscription     *pubsub.Subscription
	AIClient         *documentai.DocumentProcessorClient
	AIProcessorName  string
	DstBucketName    string
	RefsBucketHandle *storage.BucketHandle
}

// NewOCRWorkerSvc creates an instance of the OCRWorkerSvc Service.
func NewOCRWorkerSvc(ctx context.Context, o *SvcOptions) OCRWorkerSvc {
	return &ocrWorkerSvc{
		ready:            false,
		Context:          ctx,
		Topic:            o.Topic,
		Subscription:     o.Subscription,
		AIClient:         o.AIClient,
		AIProcessorName:  o.AIProcessorName,
		DstBucketName:    o.DstBucketName,
		RefsBucketHandle: o.RefsBucketHandle,
	}
}

// IsReady returns a bool describing the state of the service.
// Output:
//
//	True when the service is processing SQS messages
//	Otherwise False
func (svc *ocrWorkerSvc) IsReady() bool {
	return svc.ready
}

func existsInRefsBucket(ctx context.Context, bucket *storage.BucketHandle, filename string) (bool, error) {
	_, err := bucket.Object(filename).NewReader(ctx)
	if err != nil && err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		log.Error().Err(err).Msg("failed to check refs bucket")
	}

	return true, nil
}

// Start is the main business logic loop.
func (svc *ocrWorkerSvc) Start() error {
	svc.ready = true
	log.Info().Msg("service started")

	msgHandler := func(ctx context.Context, m *pubsub.Message) {
		start := time.Now()

		var filenames []string
		if err := utils.DecodeFromBase64(&filenames, string(m.Data)); err != nil {
			// todo: write to err bucket
			m.Nack()
			return
		}

		// Acknowledge the message
		m.Ack()

		documents := formatDocs(ctx, svc.RefsBucketHandle, filenames)
		req := formatDocAIReq(svc.AIProcessorName, svc.DstBucketName, documents)

		success, failures, err := submitDocAIBatch(ctx, svc.AIClient, req)
		if err != nil {
			log.Error().Err(err).Msg("catastrophic failure")
		}

		// write refs to refs bucket
		if errs := writeRefs(ctx, svc.RefsBucketHandle, success); len(errs) > 0 {
			log.Printf("failed to write refs: %v", err)
		}

		// the OCR work rate will control the NLP rate, which is the rate limiting factor.
		// the duration of this work must be >- 60 secs
		elapsed := time.Since(start)

		if elapsed.Seconds() < 60 {
			sleepDuration := 60 - elapsed.Seconds() + 3 // 5% buffer
			time.Sleep(time.Duration(sleepDuration) * time.Second)
		}

		log.Info().
			Int("failures", len(failures)).
			Int("success", len(success)).
			Float64("ocr duration", elapsed.Seconds()).
			Float64("total time", time.Since(start).Seconds()).
			Msgf("%d/%d", len(success), len(filenames))
	}

	// Main service loop.
	for svc.ready {
		if err := svc.Subscription.Receive(svc.Context, msgHandler); err != nil {
			log.Print("msg", "failed to receive message", "error", err)
		}
	}

	log.Info().Msg("service task completed")
	return nil
}

func writeRefs(ctx context.Context, bucket *storage.BucketHandle, docs []string) []error {
	var errs []error
	for _, d := range docs {
		if _, err := writeRef(ctx, bucket, utils.GetFilenameFromPath(d), d); err != nil {
			log.Printf("failed to write ref: %v", err)
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

// Stop instructs the service to stop processing new messages.
func (svc *ocrWorkerSvc) Stop() {
	log.Info().Msg("stopping service")
	svc.ready = false
}

func formatDocs(ctx context.Context, b *storage.BucketHandle, filenames []string) []*documentaipb.GcsDocument {
	var documents []*documentaipb.GcsDocument

	for i, f := range filenames {
		// check if file exists in refs bucket
		if ok, err := existsInRefsBucket(ctx, b, f); err != nil || ok {
			// todo: if err write to src-err
			continue
		}

		mime := ""
		mime, err := utils.GetMimeTypeFromExt(f)
		if err != nil {
			mime = "image/jpeg"
		}
		documents[i] = &documentaipb.GcsDocument{
			GcsUri:   f,
			MimeType: mime,
		}
	}
	return documents
}

func submitDocAIBatch(ctx context.Context, client *documentai.DocumentProcessorClient, req *documentaipb.BatchProcessRequest) ([]string, []string, error) {
	var success []string
	var failures []string

	// process request
	op, err := client.BatchProcessDocuments(ctx, req)
	if err != nil {
		return success, failures, fmt.Errorf("op: %w", err)
	}

	// Handle the results.
	_, err = op.Wait(ctx)

	// get metadata
	meta, metaErr := op.Metadata()
	if metaErr != nil {
		return success, failures, fmt.Errorf("meta: %w", metaErr)
	}

	// log the metadata
	// log.Info().Msg("BatchProcessDocuments", op.Name(), meta.State, meta.UpdateTime)

	// https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
	// log individual process status
	for _, i := range meta.IndividualProcessStatuses {
		if i.Status.Code == 0 {
			success = append(success, i.InputGcsSource)
		} else {
			failures = append(failures, i.InputGcsSource)
			log.Error().Err(errors.New(i.Status.Message)).Int32("StatusCode", i.Status.Code).Str("file", i.InputGcsSource).Msgf("failed to process %s", i.InputGcsSource)

		}
	}

	if err != nil {
		return success, failures, err
	}

	return success, failures, nil
}

func formatDocAIReq(processorName string, target string, documents []*documentaipb.GcsDocument) *documentaipb.BatchProcessRequest {
	// https://pkg.go.dev/cloud.google.com/go/documentai/apiv1/documentaipb#ProcessRequest
	return &documentaipb.BatchProcessRequest{
		Name:            processorName,
		SkipHumanReview: true,
		InputDocuments: &documentaipb.BatchDocumentsInputConfig{
			Source: &documentaipb.BatchDocumentsInputConfig_GcsDocuments{
				GcsDocuments: &documentaipb.GcsDocuments{
					Documents: documents,
				},
			},
		},
		DocumentOutputConfig: &documentaipb.DocumentOutputConfig{
			Destination: &documentaipb.DocumentOutputConfig_GcsOutputConfig_{
				GcsOutputConfig: &documentaipb.DocumentOutputConfig_GcsOutputConfig{
					GcsUri: fmt.Sprintf("gs://%s", target),
				},
			},
		},
	}
}
