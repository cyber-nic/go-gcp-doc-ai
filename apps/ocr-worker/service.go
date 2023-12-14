package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	documentai "cloud.google.com/go/documentai/apiv1"
	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/ocr-worker/libs/utils"
)

// SvcOptions is the representation of the options availble to the OCRWorkerSvc service
type SvcOptions struct {
	Topic                   *pubsub.Topic
	Subscription            *pubsub.Subscription
	AIClient                *documentai.DocumentProcessorClient
	AIProcessorName         string
	DstBucketName           string
	ErrBucketHandle         *storage.BucketHandle
	RefsBucketHandle        *storage.BucketHandle
	DocAIMinAsyncReqSeconds int
}

// OCRWorkerSvc is the interface for the ocrWorkerSvc service.
type OCRWorkerSvc interface {
	IsReady() bool
	Start() error
	Stop()
}

// ocrWorkerSvc is a service that will submit a batch of documents to the Document AI API.
type ocrWorkerSvc struct {
	ready                   bool
	Context                 context.Context
	Topic                   *pubsub.Topic
	Subscription            *pubsub.Subscription
	AIClient                *documentai.DocumentProcessorClient
	AIProcessorName         string
	DstBucketName           string
	ErrBucketHandle         *storage.BucketHandle
	RefsBucketHandle        *storage.BucketHandle
	DocAIMinAsyncReqSeconds float64
}

// NewOCRWorkerSvc creates an instance of the OCRWorkerSvc Service.
func NewOCRWorkerSvc(ctx context.Context, o *SvcOptions) OCRWorkerSvc {
	return &ocrWorkerSvc{
		ready:                   false,
		Context:                 ctx,
		Topic:                   o.Topic,
		Subscription:            o.Subscription,
		AIClient:                o.AIClient,
		AIProcessorName:         o.AIProcessorName,
		DstBucketName:           o.DstBucketName,
		ErrBucketHandle:         o.ErrBucketHandle,
		RefsBucketHandle:        o.RefsBucketHandle,
		DocAIMinAsyncReqSeconds: float64(o.DocAIMinAsyncReqSeconds),
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
		log.Error().Err(err).Caller().Msg("failed to check refs bucket")
	}

	return true, nil
}

// Start is the main business logic loop.
func (svc *ocrWorkerSvc) Start() error {
	svc.ready = true

	msgHandler := func(ctx context.Context, m *pubsub.Message) {
		start := time.Now()

		var filenames []string
		if err := utils.DecodeFromBase64(&filenames, string(m.Data)); err != nil {
			// todo: write to err bucket
			m.Nack()
			return
		}

		// acknowledge message
		m.Ack()
		log.Info().Int("files", len(filenames)).Caller().Msgf("msg acknowledged. processing %d files", len(filenames))

		// convert []string into []*documentaipb.GcsDocument
		documents := formatDocs(ctx, svc.RefsBucketHandle, filenames)
		// build *documentaipb.BatchProcessRequest
		req := formatDocAIReq(svc.AIProcessorName, svc.DstBucketName, documents)

		// perform batch OCR request
		success, failures, err := submitDocAIBatch(ctx, svc.AIClient, req)
		if err != nil && err.Error() != "rpc error: code = InvalidArgument desc = Failed to process all documents." {
			log.Error().Err(err).Caller().Msgf("error submitting batch: %v", err)
		}

		// write success refs
		if errs := writeKVRefs(ctx, svc.RefsBucketHandle, success); len(errs) > 0 {
			for _, e := range errs {
				log.Error().Err(e).Caller().Msg("failed to write success ref")
			}
		}

		// write failure errs
		if errs := writeKVRefs(ctx, svc.ErrBucketHandle, failures); len(errs) > 0 {
			for _, e := range errs {
				log.Error().Err(e).Caller().Msg("failed to write error")
			}
		}

		// the OCR work rate will control the NLP rate, which is the rate limiting factor.
		elapsed := time.Since(start)

		// sleep if the elapsed time is less than x seconds
		if elapsed.Seconds() < svc.DocAIMinAsyncReqSeconds {
			sleepDuration := svc.DocAIMinAsyncReqSeconds - elapsed.Seconds()
			time.Sleep(time.Duration(sleepDuration) * time.Second)
		}
		total := time.Since(start).Seconds()

		// log the results as info or error if there are failures
		l := func() *zerolog.Event {
			if len(failures) > 0 {
				return log.Error()
			} else {
				return log.Info()
			}
		}()
		l.Caller().
			Int("failures", len(failures)).
			Int("success", len(success)).
			Float64("ocr duration", elapsed.Seconds()).
			Float64("total time", total).
			Msgf("processed %d/%d files in %f seconds", len(success), len(filenames), total)
	}

	// Main service loop.
	for svc.ready {
		if err := svc.Subscription.Receive(svc.Context, msgHandler); err != nil {
			log.Error().Err(err).Caller().Msg("failed to receive message")
		}
	}

	log.Info().Msg("service task completed")
	return nil
}

func writeKVRefs(ctx context.Context, bucket *storage.BucketHandle, docs []KV) []error {
	var errs []error
	for _, kv := range docs {
		if _, err := writeRef(ctx, bucket, kv.Key, kv.Value); err != nil {
			log.Error().Err(err).Caller().Msg("failed to write ref")
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

	for _, f := range filenames {
		// check if file exists in refs bucket
		if ok, err := existsInRefsBucket(ctx, b, utils.GetFilenameFromPath(f)); err != nil || ok {
			// todo: if err write to src-err
			continue
		}

		mime := ""
		mime, err := utils.GetMimeTypeFromExt(f)
		if err != nil {
			mime = "image/jpeg"
		}
		documents = append(documents, &documentaipb.GcsDocument{
			GcsUri:   f,
			MimeType: mime,
		})
	}
	return documents
}

type KV struct {
	Key   string
	Value string
}

func submitDocAIBatch(
	ctx context.Context,
	client *documentai.DocumentProcessorClient,
	req *documentaipb.BatchProcessRequest,
) ([]KV, []KV, error) {
	var success []KV
	var failures []KV

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

	// https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
	// log individual process status
	for _, i := range meta.IndividualProcessStatuses {
		filename := strings.Replace(i.InputGcsSource, "gs://", "", 1)
		if i.Status.Code == 0 {
			success = append(success, KV{Key: filename, Value: ""})
		} else {
			failures = append(failures, KV{Key: fmt.Sprintf("%s.log", filename), Value: i.Status.Message})
			// log
			log.Error().Err(errors.New(i.Status.Message)).Caller().
				Int32("StatusCode", i.Status.Code).
				Str("file", i.InputGcsSource).
				Msgf("failed to process %s", i.InputGcsSource)
		}
	}

	if err != nil {
		return success, failures, err
	}

	return success, failures, nil
}

func formatDocAIReq(proc string, target string, docs []*documentaipb.GcsDocument) *documentaipb.BatchProcessRequest {
	// https://pkg.go.dev/cloud.google.com/go/documentai/apiv1/documentaipb#ProcessRequest
	return &documentaipb.BatchProcessRequest{
		Name:            proc,
		SkipHumanReview: true,
		InputDocuments: &documentaipb.BatchDocumentsInputConfig{
			Source: &documentaipb.BatchDocumentsInputConfig_GcsDocuments{
				GcsDocuments: &documentaipb.GcsDocuments{
					Documents: docs,
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
