// Package utils provides utility functions for the application
package utils

import (
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestGetIntEnvVar(t *testing.T) {
	tests := map[string]struct {
		key      string
		fallback int
		value    string
		expect   int
	}{
		// happy path. env var is set
		"value":    {key: "FOO_BAR", expect: 1, fallback: 1, value: "1"},
		// env var is not set. fallback is returned
		"fallback": {key: "BAR_FOO", expect: 10, fallback: 10},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// set env var if value is not empty
			if ok := tc.value != ""; ok {
				os.Setenv(tc.key, tc.value)
				defer os.Unsetenv(tc.key)
			}
			// test
			res := GetIntEnvVar(tc.key, tc.fallback)
			if !reflect.DeepEqual(tc.expect, res) {
				t.Fatalf("expected: %v, result: %v", tc.expect, res)
			}
		})
	}
}

func TestGetStrEnvVar(t *testing.T) {
	tests := map[string]struct {
		key      string
		fallback string
		value    string
		expect   string
	}{
		// happy path. env var is set
		"value":    {key: "FOO_BAR", expect: "foo", fallback: "bar", value: "foo"},
		// env var is not set. fallback is returned
		"fallback": {key: "BAR_FOO", expect: "bar", fallback: "bar"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// set env var if value is not empty
			if ok := tc.value != ""; ok {
				os.Setenv(tc.key, tc.value)
				defer os.Unsetenv(tc.key)
			}
			// test
			res := GetStrEnvVar(tc.key, tc.fallback)
			if !reflect.DeepEqual(tc.expect, res) {
				t.Fatalf("expected: %v, result: %v", tc.expect, res)
			}
		})
	}
}



func TestGetBoolEnvVar(t *testing.T) {
	tests := map[string]struct {
		key      string
		fallback bool
		value    bool
		expect   bool
	}{
		// happy path. env var is set
		"value":    {key: "FOO_BAR", expect: true, fallback: false, value: true},
		// env var is not set. fallback is returned
		"fallback": {key: "BAR_FOO", expect: true, fallback: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// set env var if value is not empty
			if ok := tc.value == true; ok {
				os.Setenv(tc.key, strconv.FormatBool(tc.value))
				defer os.Unsetenv(tc.key)
			}
			// test
			res := GetBoolEnvVar(tc.key, tc.fallback)
			if !reflect.DeepEqual(tc.expect, res) {
				t.Fatalf("expected: %v, result: %v", tc.expect, res)
			}
		})
	}
}


