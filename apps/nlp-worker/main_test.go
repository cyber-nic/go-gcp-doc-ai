package worker

import (
	"testing"

	"context"
	"io/ioutil"
	"log"
	"os"

	"github.com/cloudevents/sdk-go/v2/event"
)

// https://cloud.google.com/functions/docs/samples/functions-cloudevent-pubsub-unit-test

func TestHelloPubSub(t *testing.T) {
	tests := []struct {
		data string
		want string
	}{
		{want: "Hello, World!\n"},
		{data: "Go", want: "Hello, Go!\n"},
	}
	for _, test := range tests {
		r, w, _ := os.Pipe()
		log.SetOutput(w)
		originalFlags := log.Flags()
		log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

		m := PubSubMessage{
			Data: []byte(test.data),
		}
		msg := MessagePublishedData{
			Message: m,
		}
		e := event.New()
		e.SetDataContentType("application/json")
		e.SetData(e.DataContentType(), msg)

		helloPubSub(context.Background(), e)

		w.Close()
		log.SetOutput(os.Stderr)
		log.SetFlags(originalFlags)

		out, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if got := string(out); got != test.want {
			t.Errorf("HelloPubSub(%q) = %q, want %q", test.data, got, test.want)
		}
	}
}

// import (
// 	"encoding/json"
// 	"net/http/httptest"
// 	"strings"
// 	"testing"

// 	"github.com/google/go-cmp/cmp"

// 	"cloud.google.com/go/storage"
// 	"github.com/cyber-nic/go-gcp-doc-ai/types"
// )

// var n = storage.ObjectAttrs{
// 	Bucket: "YOUR_BUCKET_NAME",
// 	Name:   "YOUR_OBJECT_NAME",
// 	CRC32C: 1234,
// }

// func TestEntrypoint(t *testing.T) {
// 	bucket := "myBucketName"
// 	tests := []struct {
// 		body types.WorkerData
// 		want string
// 	}{
// 		{body: types.WorkerData{}, want: "Hello, World!"},
// 		{body: types.WorkerData{Data: []storage.ObjectAttrs{}}, want: "Hello, Gopher!"},
// 		{body: types.WorkerData{Data: []storage.ObjectAttrs{
// 			{
// 				Bucket: bucket,
// 				Name:   "myObjectName",
// 				CRC32C: 1234,
// 			},
// 		}}, want: "Hello, Gopher!"},
// 	}

// 	for _, test := range tests {
// 		data, err := json.Marshal(test.body)
//     if err != nil {
//         panic (err)
//     }
// 		req := httptest.NewRequest("POST", "/worker", strings.NewReader(string(data)))
// 		req.Header.Add("Content-Type", "application/json")

// 		rr := httptest.NewRecorder()
// 		Entrypoint(rr, req)

// 		diff := cmp.Diff(rr.Body.String(), test.want)
// 		if diff != "" {
// 			t.Errorf("unexpected output. %v", diff)
// 		}
// 	}
// }
