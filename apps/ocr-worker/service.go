package main

import (
	"context"
	"log"

	"cloud.google.com/go/pubsub"
	"github.com/cyber-nic/go-gcp-docai-ocr/libs/utils"
)

// SvcOptions is the representation of the options availble to the OCRWorkerSvc service
type SvcOptions struct {
	Topic        *pubsub.Topic
	Subscription *pubsub.Subscription
}

type OCRWorkerSvc interface {
	IsReady() bool
	Start() error
	Stop()
}

// OCRWorkerSvc is a generic service
type ocrWorkerSvc struct {
	ready        bool
	Context      context.Context
	Topic        *pubsub.Topic
	Subscription *pubsub.Subscription
}

// NewOCRWorkerSvc creates an instance of the OCRWorkerSvc Service.
func NewOCRWorkerSvc(ctx context.Context, o *SvcOptions) OCRWorkerSvc {
	return &ocrWorkerSvc{
		ready:        false,
		Context:      ctx,
		Topic:        o.Topic,
		Subscription: o.Subscription,
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

		utils.PrintStruct(filenames)

	}

	// Main service loop.
	for svc.ready {
		if err := svc.Subscription.Receive(svc.Context, msgHandler); err != nil {
			log.Printf("msg", "failed to receive message", "error", err)
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
