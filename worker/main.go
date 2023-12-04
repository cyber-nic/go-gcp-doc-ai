package functions

import (
	"fmt"
	"net/http"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

func init() {
    // Register HTTP function with the Functions Framework
    functions.HTTP("worker", Entrypoint)
}

// Function workerEntrypoint is an HTTP handler
func Entrypoint(w http.ResponseWriter, r *http.Request) {
    // Your code here

    // Send an HTTP response
    fmt.Fprintln(w, "OK")
}