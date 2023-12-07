// Package utils provides utility functions for the application
package utils

import (
	"log"
	"os"
	"strconv"
)

// GetIntEnvVar returns an int from an environment variable
func GetIntEnvVar(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err != nil {
			log.Fatal("Invalid value for environment variable: " + key)
		}
		return i
	}
	return fallback
}

// GetStrEnvVar returns a string from an environment variable
func GetStrEnvVar(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// GetBoolEnvVar returns a bool from an environment variable
func GetBoolEnvVar(key string, fallback bool) bool {
	val := GetStrEnvVar(key, strconv.FormatBool(fallback))
	ret, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return ret
}
