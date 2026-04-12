// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatConnectionError(t *testing.T) {
	tests := []struct {
		name           string
		inputError     error
		expectedSubstr string
	}{
		{
			name:           "AWS exec error",
			inputError:     errors.New("exec: \"aws\": executable file not found in $PATH"),
			expectedSubstr: "AWS CLI not found",
		},
		{
			name:           "gcloud exec error",
			inputError:     errors.New("exec: 'gcloud': executable file not found"),
			expectedSubstr: "gcloud CLI not found",
		},
		{
			name:           "Azure exec error",
			inputError:     errors.New("exec: \"az\": executable file not found in $PATH"),
			expectedSubstr: "Azure CLI not found",
		},
		{
			name:           "Generic exec error",
			inputError:     errors.New("exec: \"some-tool\": executable file not found in $PATH"),
			expectedSubstr: "credential plugin not found",
		},
		{
			name:           "Connection refused",
			inputError:     errors.New("dial tcp 127.0.0.1:6443: connection refused"),
			expectedSubstr: "cluster connection refused",
		},
		{
			name:           "Timeout error",
			inputError:     errors.New("context deadline exceeded"),
			expectedSubstr: "connection timeout",
		},
		{
			name:           "Other error",
			inputError:     errors.New("some other error"),
			expectedSubstr: "some other error",
		},
		{
			name:           "Nil error",
			inputError:     nil,
			expectedSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatConnectionError(tt.inputError)

			if tt.inputError == nil {
				if result != nil {
					t.Errorf("expected nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected error, got nil")
				return
			}

			if !strings.Contains(result.Error(), tt.expectedSubstr) {
				t.Errorf("expected error to contain %q, got %q", tt.expectedSubstr, result.Error())
			}
		})
	}
}
