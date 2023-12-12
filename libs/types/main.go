// Package types contains the types used by more then one application in this repo.
package types

import "cloud.google.com/go/storage"

// https://cloud.google.com/eventarc/docs/workflows/cloudevents
// {
//   "subscription": "projects/my-project/subscriptions/my-sub",
//   "message": {
//     "attributes": {
//       "attr1":"attr1-value"
//     },
//     "data": "aGVsbG8gd29ybGQ=",
//     "messageId": "2070443601311540",
//     "publishTime":"2021-02-26T19:13:55.749Z"
//   }
// }

// CloudEvent represents the CloudEvent message sent by EventArc.
type CloudEvent struct {
	Subscription string            `json:"subscription"`
	Message      CloudEventMessage `json:"message"`
}

// CloudEventMessage represents the CloudEvent message sent by EventArc.
type CloudEventMessage struct {
	Attributes map[string]string     `json:"attributes"`
	Data       []storage.ObjectAttrs `json:"data"`
}

// ImageDocument represents the computed hash and its associated image paths, along with additional metadata.
type ImageDocument struct {
	Hash       string   `firestore:"hash" `
	MimeType   string   `firestore:"mime_type"   json:"mime_type"`
	ImagePaths []string `firestore:"image_paths" json:"image_paths"`
	Width      int      `firestore:"width"`
	Height     int      `firestore:"height"`
	Pixels     int      `firestore:"pixels"`
	Size       int64    `firestore:"size" `
}
