// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/secrets"
)

// TestValidationExitCode_ValidIsZero verifies that when Valid is true, ExitCode is 0
func TestValidationExitCode_ValidIsZero(t *testing.T) {
	result := &secrets.ValidationResult{
		Valid:    true,
		ExitCode: 0,
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode=0 when Valid=true, got %d", result.ExitCode)
	}
}

// TestValidationExitCode_InvalidIsOne verifies that when Valid is false, ExitCode is 1
func TestValidationExitCode_InvalidIsOne(t *testing.T) {
	result := &secrets.ValidationResult{
		Valid:    false,
		ExitCode: 1,
	}

	if result.ExitCode != 1 {
		t.Errorf("Expected ExitCode=1 when Valid=false, got %d", result.ExitCode)
	}
}

// TestValidationExitCode_Consistency verifies the relationship between Valid and ExitCode
func TestValidationExitCode_Consistency(t *testing.T) {
	testCases := []struct {
		name     string
		valid    bool
		exitCode int
		wantErr  bool
	}{
		{
			name:     "valid with exit code 0",
			valid:    true,
			exitCode: 0,
			wantErr:  false,
		},
		{
			name:     "invalid with exit code 1",
			valid:    false,
			exitCode: 1,
			wantErr:  false,
		},
		{
			name:     "inconsistent: valid with exit code 1",
			valid:    true,
			exitCode: 1,
			wantErr:  true,
		},
		{
			name:     "inconsistent: invalid with exit code 0",
			valid:    false,
			exitCode: 0,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := &secrets.ValidationResult{
				Valid:    tc.valid,
				ExitCode: tc.exitCode,
			}

			// Check consistency
			expectedExitCode := 0
			if !tc.valid {
				expectedExitCode = 1
			}

			isConsistent := result.ExitCode == expectedExitCode

			if tc.wantErr && isConsistent {
				t.Errorf("Expected inconsistency but got consistent result")
			}

			if !tc.wantErr && !isConsistent {
				t.Errorf("Expected consistency but got inconsistent result: Valid=%v, ExitCode=%d, expected=%d",
					result.Valid, result.ExitCode, expectedExitCode)
			}
		})
	}
}
