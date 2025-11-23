// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law of an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewSecretsCmd(t *testing.T) {
	var (
		out    bytes.Buffer
		output string
	)

	// test "secrets" command by itself
	cmd := NewSecretsCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	output = out.String()
	if !strings.Contains(output, "Provides a Barbican-backed control plane") {
		t.Errorf("unexpected output: %s", output)
	}
}
