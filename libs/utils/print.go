package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

func PrintStruct(t interface{}) {
	j, _ := json.MarshalIndent(t, "", "  ")
	fmt.Println(string(j))
}


func GetFilenameFromPath(f string) string {
	// Split the object name into parts
	parts := strings.Split(f, "/")

	// Extract the filename
	filename := parts[len(parts)-1]

	return filename
}
