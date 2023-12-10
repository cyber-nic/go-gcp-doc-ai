package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cyber-nic/go-gcp-doc-ai/libs/types"
)

func init() {
	functions.CloudEvent("worker", processEvent)
}

// MessagePublishedData contains the full Pub/Sub message
// See the documentation for more details:
// https://cloud.google.com/eventarc/docs/cloudevents#pubsub
type MessagePublishedData struct {
	Message PubSubMessage
}

// PubSubMessage is the payload of a Pub/Sub event.
// See the documentation for more details:
// https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage
type PubSubMessage struct {
	Data []byte `json:"data"`
}

func processEvent(ctx context.Context, e event.Event) error {
	// Extract the object names from the Cloud Event
	msg, err := getCloudEventData(e)
	if err != nil {
		fmt.Printf("failed to unmarshal event data: %v\n", err)
		return err
	}

	// Print the object names
	for _, attrs := range msg.Message.Data {
		fmt.Println(attrs.Name)
	}

	return nil
}

func getCloudEventData(e event.Event) (types.CloudEvent, error) {
	var msg types.CloudEvent
	err := json.Unmarshal(e.Data(), &msg)
	if err != nil {
		fmt.Printf("failed to unmarshal event data: %v\n", err)
		return msg, err
	}
	return msg, nil
}
