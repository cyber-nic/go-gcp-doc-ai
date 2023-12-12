// Package main is the main application for the nlp-worker local cmd
package main

import (
	"log"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	_ "github.com/cyber-nic/go-gcp-doc-ai/apps/nlp-worker"
)

func main() {
	// The server will run on port 8080
	port := "8080"
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}
