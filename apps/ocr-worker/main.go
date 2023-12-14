// package main is the entry point for the ocr-worker application.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	documentai "cloud.google.com/go/documentai/apiv1"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/ocr-worker/libs/utils"
	"google.golang.org/api/option"
)

func main() {
	// app config
	cfg := getConfig()

	// context and signal handling
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	defer func() {
		signal.Stop(signalChan)
		cancel()
	}()

	// interrupt handling
	done := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM, os.Interrupt)
	}()

	// pubsub client
	c, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Fatal().Err(err).Str("project", cfg.ProjectID).Caller().Msg("failed to create pubsub client")
	}
	defer c.Close()

	// pubsub topic
	t := c.Topic(cfg.PubsubTopicID)
	if ok, err := t.Exists(ctx); err != nil || !ok {
		log.Fatal().Err(err).
			Str("project", cfg.ProjectID).
			Str("topic", cfg.PubsubTopicID).
			Msg("pubsub topic failed")
	}

	// pubsub subscription
	s := c.Subscription(cfg.PubsubSubscriptionID)
	if ok, err := s.Exists(ctx); err != nil || !ok {
		log.Fatal().Err(err).
			Str("project", cfg.ProjectID).
			Str("topic", cfg.PubsubTopicID).
			Str("subscription", cfg.PubsubSubscriptionID).
			Msg("pubsub subscription failed")
	}

	// doc ai processor
	endpoint := fmt.Sprintf("%s-documentai.googleapis.com:443", cfg.DocAIProcessorLocation)
	ai, err := documentai.NewDocumentProcessorClient(ctx, option.WithEndpoint(endpoint))
	if err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to create Document AI client")
	}
	defer ai.Close()
	// doc ai processor name
	proc := fmt.Sprintf("projects/%s/locations/%s/processors/%s", cfg.ProjectID, cfg.DocAIProcessorLocation, cfg.DocAIProcessorID)

	// create storage client
	store, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Caller().Msg("failed to create storage client")
	}
	defer store.Close()

	// err bucket
	errBucketHandle := store.Bucket(cfg.ErrBucketName)
	if _, err := errBucketHandle.Attrs(ctx); err != nil {
		log.Fatal().Err(err).Str("bucket", cfg.ErrBucketName).Caller().Msgf("failed to get bucket %s", cfg.ErrBucketName)
	}

	// ref bucket
	refsBucketHandle := store.Bucket(cfg.RefsBucketName)
	if _, err := refsBucketHandle.Attrs(ctx); err != nil {
		log.Fatal().Err(err).Str("bucket", cfg.RefsBucketName).Caller().Msgf("failed to get bucket %s", cfg.RefsBucketName)
	}

	// main service
	svc := NewOCRWorkerSvc(ctx, &SvcOptions{
		Topic:            t,
		Subscription:     s,
		AIClient:         ai,
		AIProcessorName:  proc,
		DstBucketName:    cfg.DstBucketName,
		ErrBucketHandle:  errBucketHandle,
		RefsBucketHandle: refsBucketHandle,
	})
	go func() {
		done <- svc.Start()
	}()

	// enable context cancelling
	go func() {
		select {
		case <-signalChan: // first signal, cancel context
			cancel()
			svc.Stop()
		case <-ctx.Done():
		}
		<-signalChan // second signal, hard exit
		os.Exit(2)
	}()

	// metrics and health
	startWebServer(svc, done, cfg.Port)

	// wait for exit
	<-done
	log.Info().Caller().Msg("exit")
}

func startWebServer(svc OCRWorkerSvc, exit chan error, p string) {
	go func() {
		port := ":" + p
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if svc.IsReady() {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ready"))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("not ready"))
		})
		log.Info().Str("port", port).Caller().Msg(fmt.Sprintf("Serving '/health' on port %s", port))

		server := &http.Server{
			Addr:              port,
			ReadHeaderTimeout: 30 * time.Second,
		}
		exit <- server.ListenAndServe()
	}()
}

type appConfig struct {
	Debug                  bool
	Port                   string
	ProjectID              string
	DocAIProcessorID       string
	DocAIProcessorLocation string
	DstBucketName          string
	ErrBucketName          string
	RefsBucketName         string
	PubsubTopicID          string
	PubsubSubscriptionID   string
	DocAIMaxReqPerMinute   int
}

func getMandatoryEnvVar(n string) string {
	v := os.Getenv(n)
	if v != "" {
		return v
	}
	log.Fatal().Caller().Msgf("%s required", n)
	return ""
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

	// pubsub
	pubsubTopicID := getMandatoryEnvVar("PUBSUB_TOPIC_ID")
	pubsubSubID := getMandatoryEnvVar("PUBSUB_SUBSCRIPTION_ID")

	// doc ai
	docAIProcessorID := getMandatoryEnvVar("DOC_AI_PROCESSOR_ID")
	docAIProcessorLocation := getMandatoryEnvVar("DOC_AI_PROCESSOR_LOCATION")
	// maxDocAIReqPerMinute allows for the controler of the number of doc ai requests per minute
	// to avoid exceeding the quota of downstream services such as NLP.
	docAIMaxReqPerMinute := utils.GetIntEnvVar("DOC_AI_MAX_REQ_PER_MIN", 1)

	return appConfig{
		Debug:                  debug,
		Port:                   port,
		ProjectID:              projectID,
		DstBucketName:          dstBucketName,
		RefsBucketName:         refsBucketName,
		ErrBucketName:          errBucketName,
		PubsubTopicID:          pubsubTopicID,
		PubsubSubscriptionID:   pubsubSubID,
		DocAIMaxReqPerMinute:   docAIMaxReqPerMinute,
		DocAIProcessorID:       docAIProcessorID,
		DocAIProcessorLocation: docAIProcessorLocation,
	}
}
