/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"errors"
	"strings"
	"testing"
)

// TestFormatError tests the formatError helper function
// Requirements: 15.1, 15.5, 15.6
func TestFormatError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantNil bool
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:    "simple error is formatted",
			err:     errors.New("test error"),
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatError(tt.err)
			if tt.wantNil && result != nil {
				t.Errorf("formatError() = %v, want nil", result)
			}
			if !tt.wantNil && result == nil {
				t.Errorf("formatError() = nil, want non-nil")
			}
		})
	}
}


// TestFormatErrorWithCode tests the formatErrorWithCode helper function
// Requirements: 15.1, 15.2, 15.4
func TestFormatErrorWithCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     string
		wantCode bool
	}{
		{
			name:     "nil error returns nil",
			err:      nil,
			code:     "E1001",
			wantCode: false,
		},
		{
			name:     "error with code includes code",
			err:      errors.New("test error"),
			code:     "E1001",
			wantCode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatErrorWithCode(tt.err, tt.code)
			if tt.err == nil && result != nil {
				t.Errorf("formatErrorWithCode() = %v, want nil", result)
			}
			if tt.wantCode && result != nil && !strings.Contains(result.Error(), tt.code) {
				t.Errorf("formatErrorWithCode() = %v, want to contain %s", result, tt.code)
			}
		})
	}
}

// TestFormatErrorWithInfo tests the formatErrorWithInfo helper function
// Requirements: 15.1, 15.2, 15.3, 15.4, 15.8
func TestFormatErrorWithInfo(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		code         string
		wantNil      bool
		wantContains []string
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			code:    "E1001",
			wantNil: true,
		},
		{
			name:    "known error code includes all fields",
			err:     errors.New("test error"),
			code:    "E1001",
			wantNil: false,
			wantContains: []string{
				"E1001",
				"OpenStack region not configured",
				"Fix:",
				"Learn more:",
			},
		},
		{
			name:    "unknown error code falls back to simple formatting",
			err:     errors.New("test error"),
			code:    "E9999",
			wantNil: false,
			wantContains: []string{
				"E9999",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatErrorWithInfo(tt.err, tt.code)
			if tt.wantNil && result != nil {
				t.Errorf("formatErrorWithInfo() = %v, want nil", result)
			}
			if !tt.wantNil && result == nil {
				t.Errorf("formatErrorWithInfo() = nil, want non-nil")
			}
			if result != nil {
				errStr := result.Error()
				for _, want := range tt.wantContains {
					if !strings.Contains(errStr, want) {
						t.Errorf("formatErrorWithInfo() = %v, want to contain %s", errStr, want)
					}
				}
			}
		})
	}
}

// TestFormatMultipleErrors tests the formatMultipleErrors helper function
// Requirements: 15.5, 15.6
func TestFormatMultipleErrors(t *testing.T) {
	tests := []struct {
		name         string
		errs         []error
		verbose      bool
		wantNil      bool
		wantContains string
	}{
		{
			name:    "empty error list returns nil",
			errs:    []error{},
			verbose: false,
			wantNil: true,
		},
		{
			name: "multiple errors with limit shows 'more errors'",
			errs: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
				errors.New("error 4"),
				errors.New("error 5"),
				errors.New("error 6"),
			},
			verbose:      false,
			wantNil:      false,
			wantContains: "more errors",
		},
		{
			name: "verbose mode shows all errors",
			errs: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
				errors.New("error 4"),
				errors.New("error 5"),
				errors.New("error 6"),
			},
			verbose:      true,
			wantNil:      false,
			wantContains: "error 6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMultipleErrors(tt.errs, tt.verbose)
			if tt.wantNil && result != nil {
				t.Errorf("formatMultipleErrors() = %v, want nil", result)
			}
			if !tt.wantNil && result == nil {
				t.Errorf("formatMultipleErrors() = nil, want non-nil")
			}
			if result != nil && tt.wantContains != "" {
				if !strings.Contains(result.Error(), tt.wantContains) {
					t.Errorf("formatMultipleErrors() = %v, want to contain %s", result, tt.wantContains)
				}
			}
		})
	}
}
