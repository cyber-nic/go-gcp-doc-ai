package main

import (
	"context"
	"fmt"
	"log"

	documentai "cloud.google.com/go/documentai/apiv1"
	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"cloud.google.com/go/pubsub"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/ocr-worker/libs/utils"
)

// SvcOptions is the representation of the options availble to the OCRWorkerSvc service
type SvcOptions struct {
	Topic                *pubsub.Topic
	Subscription         *pubsub.Subscription
	AIClient             *documentai.DocumentProcessorClient
	AIProcessorName      string
	DestinationBucketURI string
}

type OCRWorkerSvc interface {
	IsReady() bool
	Start() error
	Stop()
}

// OCRWorkerSvc is a generic service
type ocrWorkerSvc struct {
	ready                bool
	Context              context.Context
	Topic                *pubsub.Topic
	Subscription         *pubsub.Subscription
	AIClient             *documentai.DocumentProcessorClient
	AIProcessorName      string
	DestinationBucketURI string
}

// NewOCRWorkerSvc creates an instance of the OCRWorkerSvc Service.
func NewOCRWorkerSvc(ctx context.Context, o *SvcOptions) OCRWorkerSvc {
	return &ocrWorkerSvc{
		ready:                false,
		Context:              ctx,
		Topic:                o.Topic,
		Subscription:         o.Subscription,
		AIClient:             o.AIClient,
		AIProcessorName:      o.AIProcessorName,
		DestinationBucketURI: o.DestinationBucketURI,
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

// Start is the main business logic loop.
func (svc *ocrWorkerSvc) Start() error {
	svc.ready = true
	log.Println("service started")

	msgHandler := func(ctx context.Context, m *pubsub.Message) {
		var filenames []string
		if err := utils.DecodeFromBase64(&filenames, string(m.Data)); err != nil {
			// todo: write to err bucket
			m.Nack()
			return
		}

		// Acknowledge the message
		m.Ack()

		documents := formatDocs(filenames)
		req := formatDocAIReq(svc.AIProcessorName, svc.DestinationBucketURI, documents)

		_, err := submitDocAIBatch(ctx, svc.AIClient, req)
		if err != nil {
			log.Println(err)
		}
	
		log.Println("done")
	}

	// Main service loop.
	for svc.ready {
		if err := svc.Subscription.Receive(svc.Context, msgHandler); err != nil {
			log.Print("msg", "failed to receive message", "error", err)
		}

	}

	log.Println("service task completed")

	return nil
}

// Stop instructs the service to stop processing new messages.
func (svc *ocrWorkerSvc) Stop() {
	log.Println("stopping service")
	svc.ready = false
}

func formatDocs(filenames []string) []*documentaipb.GcsDocument {
	documents := make([]*documentaipb.GcsDocument, len(filenames))
	for i, f := range filenames {
		mime := ""
		mime, err := utils.GetMimeTypeFromExt(f)
		if err != nil {
			mime = "image/jpeg"
		}
		documents[i] = &documentaipb.GcsDocument{
			GcsUri: f,
			MimeType: mime,
		}
	}
	return documents
}

func submitDocAIBatch(ctx context.Context, client *documentai.DocumentProcessorClient, req *documentaipb.BatchProcessRequest) (string, error) {
		// process request
		op, err := client.BatchProcessDocuments(ctx, req)
		if err != nil {
			return "", fmt.Errorf("op: %w", err)
		}
		log.Println(op.Metadata())


		// Handle the results.
		resp, err := op.Wait(ctx)
		if err != nil {
			return "", fmt.Errorf("wait: %w", err)
		}
		// TODO: Use resp.
		return resp.String(), nil

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
					GcsUri: target,
				},
			},
		},
	}
}