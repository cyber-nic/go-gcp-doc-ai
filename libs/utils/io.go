package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
)

// PrintStruct prints a struct as JSON.
func PrintStruct(t interface{}) {
	j, _ := json.MarshalIndent(t, "", "  ")
	fmt.Println(string(j))
}

// GetFilenameFromPath returns the filename from a given path.
func GetFilenameFromPath(f string) string {
	// Split the object name into parts
	parts := strings.Split(f, "/")

	// Extract the filename
	filename := parts[len(parts)-1]

	return filename
}

// GetMimeTypeFromExt returns the mime type for a given file extension.
func GetMimeTypeFromExt(name string) (string, error) {
	var m string
	ext := filepath.Ext(name)
	m = mime.TypeByExtension(ext)
	if m == "" {
		return m, fmt.Errorf("failed to get mime type for %s", name)
	}

	return m, nil
}

// SetBucketFileValue writes a value to an object handle. It will create the object if it does not exist.
func SetBucketFileValue(ctx context.Context, o *storage.ObjectHandle, v string) error {
	// open writer
	w := o.NewWriter(ctx)
	defer w.Close()
	// update checkpoint
	if _, err := w.Write([]byte(v)); err != nil {
		return fmt.Errorf("(%s) failed to write: %v", o.ObjectName(), err)
	}
	// close writer
	if err := w.Close(); err != nil {
		return fmt.Errorf("(%s) failed to close writer: %v", o.ObjectName(), err)
	}
	return nil
}

// GetValueFromBucketFile reads the value from an object handle
func GetValueFromBucketFile(ctx context.Context, o *storage.ObjectHandle) string {
	_, err := o.NewReader(ctx)
	// create missing file
	if err != nil && err == storage.ErrObjectNotExist {
		w := o.NewWriter(ctx)
		defer w.Close()
		if _, err := w.Write([]byte("")); err != nil {
			log.Fatalf("failed to write %s: %v\n", o.ObjectName(), err)
		}

		// Close writer
		if err := w.Close(); err != nil {
			log.Fatalf("failed to close object writer: %v", err)
		}
	} else if err != nil {
		// fail is unexpected error
		log.Fatalf("failed to create %s reader: %v", o.ObjectName(), err)
	}

	r, err := o.NewReader(ctx)
	if err != nil {
		log.Fatalf("failed to create %s reader: %v", o.ObjectName(), err)
	}

	// var w
	var b []byte

	// Read the entire object into a byte slice.
	b, err = io.ReadAll(r)
	if err != nil {
		log.Fatalf("failed to read object: %v", err)
	}

	// Close the reader.
	if err := r.Close(); err != nil {
		log.Fatalf("failed to close object reader: %v", err)
	}

	return string(b)
}
