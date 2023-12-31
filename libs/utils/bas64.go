package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// EncodeToBase64 encodes a struct to base64
func EncodeToBase64(v interface{}) (string, error) {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := json.NewEncoder(encoder).Encode(v); err != nil {
		return "", err
	}
	encoder.Close()
	return buf.String(), nil
}

// DecodeFromBase64 decodes a struct from base64
func DecodeFromBase64(v interface{}, enc string) error {
	return json.NewDecoder(base64.NewDecoder(base64.StdEncoding, strings.NewReader(enc))).Decode(v)
}
