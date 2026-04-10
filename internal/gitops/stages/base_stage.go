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

package stages

import (
	"context"
	"fmt"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

// BaseStage provides common functionality for generation stages.
// Concrete stages can embed this to inherit default behavior.
type BaseStage struct {
	name         string
	description  string
	dependencies []string
}

// NewBaseStage creates a new base stage with the given properties.
func NewBaseStage(name, description string, dependencies []string) BaseStage {
	if dependencies == nil {
		dependencies = []string{}
	}
	return BaseStage{
		name:         name,
		description:  description,
		dependencies: dependencies,
	}
}

// Name returns the stage name.
func (bs BaseStage) Name() string {
	return bs.name
}

// Description returns the stage description.
func (bs BaseStage) Description() string {
	return bs.description
}

// Dependencies returns the stage dependencies.
func (bs BaseStage) Dependencies() []string {
	return bs.dependencies
}

// Execute is a default implementation that does nothing.
// Concrete stages should override this method.
func (bs BaseStage) Execute(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	return fmt.Errorf("stage %s does not implement Execute", bs.name)
}

// Rollback is a default implementation that does nothing.
// Concrete stages should override this method if they need rollback logic.
func (bs BaseStage) Rollback(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Default: no-op rollback
	return nil
}

// Validate is a default implementation that always succeeds.
// Concrete stages should override this method if they need validation logic.
func (bs BaseStage) Validate(ctx context.Context, workspace *gitops.GitOpsWorkspace) error {
	// Default: always valid
	return nil
}

// DryRun is a default implementation that returns an empty plan.
// Concrete stages should override this method.
func (bs BaseStage) DryRun(ctx context.Context, cfg v2.Config) (*gitops.StagePlan, error) {
	return &gitops.StagePlan{
		Name:         bs.name,
		Description:  bs.description,
		Files:        []string{},
		Directories:  []string{},
		Dependencies: bs.dependencies,
	}, nil
}
