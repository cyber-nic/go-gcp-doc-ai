package utils

import (
	"encoding/json"
	"fmt"
)

func PrintStruct(t interface{}) {
	j, _ := json.MarshalIndent(t, "", "  ")
	fmt.Println(string(j))
}
