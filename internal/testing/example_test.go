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

package testing_test

import (
	"context"
	"testing"

	testing_framework "github.com/rackerlabs/openCenter-cli/internal/testing"
)

// ExampleTestFramework demonstrates how to use the test framework in your tests.
func ExampleTestFramework() {
	// This would normally be in a test function
	t := &testing.T{}

	// Create a new test framework
	fw := testing_framework.NewTestFramework(t)

	// Generate a test configuration
	cfg := fw.CreateTestConfig("openstack")

	// Use the configuration in your tests
	_ = cfg.OpenCenter.Meta.Name

	// Write a template for testing
	templatePath := fw.WriteTemplate(t, "test.tmpl", "Hello {{ .Name }}!")

	// Render the template
	data := map[string]interface{}{"Name": "World"}
	_, _ = fw.TemplateEngine.Render(context.Background(), templatePath, data)

	// Assert files and directories exist
	fw.AssertDirExists(t, fw.TempDir)
	fw.AssertFileExists(t, templatePath)
}

// ExampleNewTestFrameworkWithSeed demonstrates how to use the test framework with a custom seed.
func ExampleNewTestFrameworkWithSeed() {
	t := &testing.T{}

	// Create a test framework with a specific seed for reproducible tests
	fw := testing_framework.NewTestFrameworkWithSeed(t, 12345)

	// Generate deterministic test data
	cfg := fw.CreateTestConfig("openstack")
	_ = cfg.OpenCenter.Meta.Name

	// Generate template data
	templateData := fw.CreateTestTemplateData()
	_ = templateData["ClusterName"]

	// Generate service definition
	service := fw.CreateTestServiceDefinition()
	_ = service["name"]

	// Generate GitOps config
	gitops := fw.CreateTestGitOpsConfig()
	_ = gitops["repository"]
}
