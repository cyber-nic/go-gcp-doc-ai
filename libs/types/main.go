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

type CloudEvent struct {
	Subscription string            `json:"subscription"`
	Message      CloudEventMessage `json:"message"`
}

type CloudEventMessage struct {
	Attributes map[string]string     `json:"attributes"`
	Data       []storage.ObjectAttrs `json:"data"`
}
