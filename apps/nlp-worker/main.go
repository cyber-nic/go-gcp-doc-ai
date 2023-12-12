// Package worker is the main application for the nlp-worker service. It is triggered by a storage bucket Finalize event. It submits a file for NLP processing.
package worker

import (
	"context"
	"fmt"
	"log"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/storagedata"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	functions.CloudEvent("Handler", handler)
}

// entrypoint consumes a CloudEvent message and logs details about the changed object.
func handler(ctx context.Context, e event.Event) error {
	log.Printf("Event ID: %s", e.ID())
	log.Printf("Event Type: %s", e.Type())

	var data storagedata.StorageObjectData
	if err := protojson.Unmarshal(e.Data(), &data); err != nil {
		return fmt.Errorf("protojson.Unmarshal: %w", err)
	}

	log.Printf("Bucket: %s", data.GetBucket())
	log.Printf("File: %s", data.GetName())
	log.Printf("Metageneration: %d", data.GetMetageneration())
	log.Printf("Created: %s", data.GetTimeCreated().AsTime())
	log.Printf("Updated: %s", data.GetUpdated().AsTime())
	return nil
}

// func getCloudEventData(e event.Event) (types.CloudEvent, error) {
// 	var msg types.CloudEvent
// 	err := json.Unmarshal(e.Data(), &msg)
// 	if err != nil {
// 		fmt.Printf("failed to unmarshal event data: %v\n", err)
// 		return msg, err
// 	}
// 	return msg, nil
// }
