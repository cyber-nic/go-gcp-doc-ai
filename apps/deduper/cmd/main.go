package main

import (
	"log"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	_ "github.com/cyber-nic/go-gcp-doc-ai/apps/deduper"
	"github.com/cyber-nic/go-gcp-doc-ai/apps/deduper/libs/utils"
)

func main() {
	// The server will run on port 8081
	port := utils.GetStrEnvVar("PORT", "8083")
	log.Println("Listening on port:", port)
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}