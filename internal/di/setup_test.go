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

package di

import (
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/ui"
)

func TestSetupContainer(t *testing.T) {
	container, err := SetupContainer()
	if err != nil {
		t.Fatalf("SetupContainer() failed: %v", err)
	}
	if container == nil {
		t.Fatal("SetupContainer() returned nil container")
	}
}

func TestSetupContainer_Logger(t *testing.T) {
	container, err := SetupContainer()
	if err != nil {
		t.Fatalf("SetupContainer() failed: %v", err)
	}

	var logger *logrus.Logger
	err = container.ResolveAs("logger", &logger)
	if err != nil {
		t.Errorf("Failed to resolve logger: %v", err)
	}
	if logger == nil {
		t.Error("Logger is nil")
	}
}

func TestSetupContainer_ConfigManager(t *testing.T) {
	container, err := SetupContainer()
	if err != nil {
		t.Fatalf("SetupContainer() failed: %v", err)
	}

	var configManager *config.ConfigManager
	err = container.ResolveAs("configManager", &configManager)
	if err != nil {
		t.Errorf("Failed to resolve configManager: %v", err)
	}
	if configManager == nil {
		t.Error("ConfigManager is nil")
	}
}

func TestSetupContainer_ErrorFormatter(t *testing.T) {
	container, err := SetupContainer()
	if err != nil {
		t.Fatalf("SetupContainer() failed: %v", err)
	}

	var errorFormatter ui.ErrorFormatter
	err = container.ResolveAs("errorFormatter", &errorFormatter)
	if err != nil {
		t.Errorf("Failed to resolve errorFormatter: %v", err)
	}
	if errorFormatter == nil {
		t.Error("ErrorFormatter is nil")
	}
}

func TestSetupContainer_Singletons(t *testing.T) {
	container, err := SetupContainer()
	if err != nil {
		t.Fatalf("SetupContainer() failed: %v", err)
	}

	// Resolve logger twice
	logger1, err := container.Resolve("logger")
	if err != nil {
		t.Errorf("Failed to resolve logger first time: %v", err)
	}

	logger2, err := container.Resolve("logger")
	if err != nil {
		t.Errorf("Failed to resolve logger second time: %v", err)
	}

	// Should be the same instance
	if logger1 != logger2 {
		t.Error("Logger should be a singleton (same instance)")
	}
}
