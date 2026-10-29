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

package cluster

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// stubDestroyProvider returns a fixed step list so the runner's severity
// contract can be tested without any provider internals.
type stubDestroyProvider struct {
	steps []destroyStep
}

func (p *stubDestroyProvider) BuildSteps(_ *v2.Config, _ *DestroyInfraOptions) ([]destroyStep, error) {
	return p.steps, nil
}

// capturingDestroyProvider records the options the service passes down.
type capturingDestroyProvider struct {
	capture **DestroyInfraOptions
}

func (p *capturingDestroyProvider) BuildSteps(_ *v2.Config, opts *DestroyInfraOptions) ([]destroyStep, error) {
	*p.capture = opts
	return nil, nil
}

func TestDestroyService_NonFatalStepRecordsWarningAndContinues(t *testing.T) {
	var ran []string
	svc := NewDestroyServiceWithRunner(&fakeDestroyCommandRunner{}, &bytes.Buffer{})
	svc.providerOverride = &stubDestroyProvider{steps: []destroyStep{
		{ID: "cleanup-alpha", Description: "alpha", Run: func(context.Context) error {
			ran = append(ran, "alpha")
			return errors.New("alpha leaked")
		}},
		{ID: "beta", Description: "beta", Fatal: true, Run: func(context.Context) error {
			ran = append(ran, "beta")
			return nil
		}},
	}}

	result, err := svc.DestroyInfrastructure(context.Background(), &v2.Config{OpenCenter: v2.OpenCenterConfig{}}, nil)
	if err != nil {
		t.Fatalf("non-fatal step must not abort destroy: %v", err)
	}
	if len(ran) != 2 {
		t.Fatalf("expected both steps to run, got %v", ran)
	}
	if len(result.CleanupWarnings) != 1 || !strings.Contains(result.CleanupWarnings[0].Error(), "alpha leaked") {
		t.Fatalf("expected alpha warning recorded, got %v", result.CleanupWarnings)
	}
	if !result.InfraDestroyed {
		t.Fatal("InfraDestroyed must be true when only non-fatal steps failed")
	}
	if result.CleanupWarning() == nil {
		t.Fatal("CleanupWarning() must aggregate recorded warnings")
	}
}

func TestDestroyService_FatalStepAbortsPipeline(t *testing.T) {
	var ran []string
	svc := NewDestroyServiceWithRunner(&fakeDestroyCommandRunner{}, &bytes.Buffer{})
	svc.providerOverride = &stubDestroyProvider{steps: []destroyStep{
		{ID: "fatal-step", Description: "fatal", Fatal: true, Run: func(context.Context) error {
			ran = append(ran, "fatal")
			return errors.New("boom")
		}},
		{ID: "never", Description: "never", Run: func(context.Context) error {
			ran = append(ran, "never")
			return nil
		}},
	}}

	result, err := svc.DestroyInfrastructure(context.Background(), &v2.Config{OpenCenter: v2.OpenCenterConfig{}}, nil)
	if err == nil {
		t.Fatal("fatal step must abort destroy with an error")
	}
	if !strings.Contains(err.Error(), "fatal-step") {
		t.Fatalf("error must name the failing step, got %v", err)
	}
	if len(ran) != 1 || ran[0] != "fatal" {
		t.Fatalf("steps after a fatal failure must not run, got %v", ran)
	}
	if result.InfraDestroyed {
		t.Fatal("InfraDestroyed must be false after a fatal failure")
	}
}

func TestDestroyService_RetryDefaults(t *testing.T) {
	svc := NewDestroyServiceWithRunner(&fakeDestroyCommandRunner{}, &bytes.Buffer{})
	var got *DestroyInfraOptions
	svc.providerOverride = &capturingDestroyProvider{capture: &got}

	testCfg := &v2.Config{OpenCenter: v2.OpenCenterConfig{}}
	if _, err := svc.DestroyInfrastructure(context.Background(), testCfg, &DestroyOptions{}); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if got == nil || got.DestroyRetries != 1 {
		t.Fatalf("default DestroyRetries must be 1, got %+v", got)
	}

	// Negative disables retries.
	if _, err := svc.DestroyInfrastructure(context.Background(), testCfg, &DestroyOptions{DestroyRetries: -1}); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if got.DestroyRetries != 0 {
		t.Fatalf("DestroyRetries=-1 must clamp to 0, got %d", got.DestroyRetries)
	}
}
