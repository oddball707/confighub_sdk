// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"errors"
	"testing"

	goclientnew "github.com/confighub/sdk/openapi/goclient-new"
)

func TestIs409Error(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "409 error with HTTP prefix",
			err:      errors.New("HTTP 409 for req abc: version conflict"),
			expected: true,
		},
		{
			name:     "409 error without HTTP prefix",
			err:      errors.New("409 conflict detected"),
			expected: true,
		},
		{
			name:     "non-409 error",
			err:      errors.New("HTTP 500 for req xyz: internal server error"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := is409Error(tt.err)
			if result != tt.expected {
				t.Errorf("is409Error() = %v, expected %v for error: %v", result, tt.expected, tt.err)
			}
		})
	}
}

func TestExtractFieldConflicts(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected []string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: nil,
		},
		{
			name: "error with field conflicts in ErrorMetadata",
			err: func() error {
				respErr := &goclientnew.ResponseError{
					Message: "409 Version conflict",
					Status:  409,
					ErrorMetadata: &goclientnew.ErrorMetadata{
						Items: []goclientnew.ErrorItem{
							{Item: "Labels", Description: "Label mismatch"},
							{Item: "Annotations", Description: "Annotation mismatch"},
						},
					},
				}
				jsonBytes, _ := json.Marshal(respErr)
				return errors.New(string(jsonBytes))
			}(),
			expected: []string{"Labels", "Annotations"},
		},
		{
			name: "error with single field conflict",
			err: func() error {
				respErr := &goclientnew.ResponseError{
					Message: "409 Version conflict",
					Status:  409,
					ErrorMetadata: &goclientnew.ErrorMetadata{
						Items: []goclientnew.ErrorItem{
							{Item: "Data", Description: "Data conflict"},
						},
					},
				}
				jsonBytes, _ := json.Marshal(respErr)
				return errors.New(string(jsonBytes))
			}(),
			expected: []string{"Data"},
		},
		{
			name:     "error without ErrorMetadata",
			err:      errors.New("HTTP 409: version conflict"),
			expected: nil,
		},
		{
			name: "error with empty Items",
			err: func() error {
				respErr := &goclientnew.ResponseError{
					Message:       "409 Version conflict",
					Status:        409,
					ErrorMetadata: &goclientnew.ErrorMetadata{Items: []goclientnew.ErrorItem{}},
				}
				jsonBytes, _ := json.Marshal(respErr)
				return errors.New(string(jsonBytes))
			}(),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFieldConflicts(tt.err)
			if len(result) != len(tt.expected) {
				t.Errorf("extractFieldConflicts() returned %d conflicts, expected %d", len(result), len(tt.expected))
				return
			}
			for i, conflict := range result {
				if conflict != tt.expected[i] {
					t.Errorf("extractFieldConflicts()[%d] = %v, expected %v", i, conflict, tt.expected[i])
				}
			}
		})
	}
}

func TestDetectClientMutableFieldChanges(t *testing.T) {
	baseUnit := &goclientnew.Unit{
		Labels:                map[string]string{"env": "prod"},
		Annotations:           map[string]string{"owner": "team-a"},
		LastChangeDescription: "Initial version",
		DisplayName:           "My Unit",
		Data:                  "original data",
		DeleteGates:           map[string]bool{"gate1": true},
		DestroyGates:          map[string]bool{"gate2": false},
	}

	tests := []struct {
		name     string
		original *goclientnew.Unit
		latest   *goclientnew.Unit
		expected []string
	}{
		{
			name:     "no changes",
			original: baseUnit,
			latest:   baseUnit,
			expected: []string{},
		},
		{
			name:     "label changed",
			original: baseUnit,
			latest: &goclientnew.Unit{
				Labels:                map[string]string{"env": "dev"},
				Annotations:           baseUnit.Annotations,
				LastChangeDescription: baseUnit.LastChangeDescription,
				DisplayName:           baseUnit.DisplayName,
				Data:                  baseUnit.Data,
				DeleteGates:           baseUnit.DeleteGates,
				DestroyGates:          baseUnit.DestroyGates,
			},
			expected: []string{"Labels"},
		},
		{
			name:     "multiple fields changed",
			original: baseUnit,
			latest: &goclientnew.Unit{
				Labels:                map[string]string{"env": "dev"},
				Annotations:           map[string]string{"owner": "team-b"},
				LastChangeDescription: "Updated version",
				DisplayName:           baseUnit.DisplayName,
				Data:                  baseUnit.Data,
				DeleteGates:           baseUnit.DeleteGates,
				DestroyGates:          baseUnit.DestroyGates,
			},
			expected: []string{"Labels", "Annotations", "LastChangeDescription"},
		},
		{
			name:     "data changed",
			original: baseUnit,
			latest: &goclientnew.Unit{
				Labels:                baseUnit.Labels,
				Annotations:           baseUnit.Annotations,
				LastChangeDescription: baseUnit.LastChangeDescription,
				DisplayName:           baseUnit.DisplayName,
				Data:                  "modified data",
				DeleteGates:           baseUnit.DeleteGates,
				DestroyGates:          baseUnit.DestroyGates,
			},
			expected: []string{"Data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectClientMutableFieldChanges(tt.original, tt.latest)
			if len(result) != len(tt.expected) {
				t.Errorf("detectClientMutableFieldChanges() returned %d conflicts, expected %d: %v", len(result), len(tt.expected), result)
				return
			}
			for i, conflict := range result {
				if conflict != tt.expected[i] {
					t.Errorf("detectClientMutableFieldChanges()[%d] = %v, expected %v", i, conflict, tt.expected[i])
				}
			}
		})
	}
}
