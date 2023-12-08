// go svc tpl
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/cyber-nic/go-gcp-docai-ocr/libs/utils"
)

const (
	exitCodeInterrupt = 2
)

var (
	debug *bool
)

func main() {
	// app config
	cfg := getConfig()
	// if cfg.Debug {
	// 	utils.PrintStruct(cfg)
	// }

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
	c, err := pubsub.NewClient(ctx, cfg.PubsubProjectID)
	if err != nil {
		log.Fatalln("failed to create pubsub client", "project", cfg.PubsubProjectID, "error", err)
		panic(err)
	}
	defer c.Close()
	log.Println("msg", "pubsub client created", "project", cfg.PubsubProjectID)

	// pubsub topic
	t := c.Topic(cfg.PubsubTopicID)
	if ok, err := t.Exists(ctx); err != nil || !ok {
		log.Fatalln("pubsub topic failed", "project", cfg.PubsubProjectID, "topic", cfg.PubsubTopicID, "error", err)
	}

	// pubsub subscription
	s := c.Subscription(cfg.PubsubSubscriptionID)
	if ok, err := s.Exists(ctx); err != nil || !ok {
		log.Fatalln("pubsub subscription failed", "project", cfg.PubsubProjectID, "subscription", cfg.PubsubSubscriptionID, "error", err)
	}

	// main service
	svc := NewOCRWorkerSvc(ctx, &SvcOptions{
		Topic:        t,
		Subscription: s,
	})
	go func() {
		if err := svc.Start(); err != nil {
			log.Fatalf("service failed: %v", err)
		}
	}()

	// allow context cancelling
	go func() {
		select {
		case <-signalChan: // first signal, cancel context
			cancel()
			svc.Stop()
		case <-ctx.Done():
		}
		<-signalChan // second signal, hard exit
		os.Exit(exitCodeInterrupt)
	}()

	// metrics and health
	// startWebServer(svc, done, cfg.Port)
	log.Println("exit", <-done)
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
		log.Print("msg", fmt.Sprintf("Serving '/health' on port %s", port))

		server := &http.Server{
			Addr:              port,
			ReadHeaderTimeout: 30 * time.Second,
		}
		exit <- server.ListenAndServe()
	}()
}

type appConfig struct {
	Debug                bool
	Port                 string
	ProjectID            string
	DstBucketName        string
	ErrBucketName        string
	RefBucketName        string
	PubsubProjectID      string
	PubsubTopicID        string
	PubsubSubscriptionID string
	MaxMsgPerMinute      int
}

func getMandatoryEnvVar(n string) string {
	v := os.Getenv(n)
	if v != "" {
		return v
	}
	log.Fatalf("%s required", n)
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
	refBucketName := getMandatoryEnvVar("REF_BUCKET_NAME")

	// pubsub
	pubsubProjectID := getMandatoryEnvVar("PUBSUB_PROJECT_ID")
	pubsubTopicID := getMandatoryEnvVar("PUBSUB_TOPIC_ID")
	pubsubSubID := getMandatoryEnvVar("PUBSUB_SUBSCRIPTION_ID")

	// maxMsgPerMinute allows for the calibration of the number of messages to be processed per minute
	// to avoid exceeding the quota of downstream services such as NLP.
	maxMsgPerMinute := utils.GetIntEnvVar("MAX_MSG_PER_MIN", 1)

	return appConfig{
		Debug:                debug,
		Port:                 port,
		ProjectID:            projectID,
		DstBucketName:        dstBucketName,
		RefBucketName:        refBucketName,
		ErrBucketName:        errBucketName,
		PubsubProjectID:      pubsubProjectID,
		PubsubTopicID:        pubsubTopicID,
		PubsubSubscriptionID: pubsubSubID,
		MaxMsgPerMinute:      maxMsgPerMinute,
	}
}
