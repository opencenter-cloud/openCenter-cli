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

package template

import (
	"context"
	"fmt"
	"sync"
)

// RenderRequest represents a template rendering request
type RenderRequest struct {
	TemplateName string
	Data         interface{}
	OutputKey    string // Optional key to identify the result
}

// RenderResult represents the result of a template rendering operation
type RenderResult struct {
	OutputKey string
	Content   string
	Error     error
}

// ParallelRenderer provides parallel template rendering capabilities
type ParallelRenderer struct {
	engine         TemplateEngine
	maxConcurrency int
}

// NewParallelRenderer creates a new parallel renderer
func NewParallelRenderer(engine TemplateEngine, maxConcurrency int) *ParallelRenderer {
	if maxConcurrency <= 0 {
		maxConcurrency = 4 // Default to 4 concurrent operations
	}
	
	return &ParallelRenderer{
		engine:         engine,
		maxConcurrency: maxConcurrency,
	}
}

// RenderMultiple renders multiple templates in parallel
func (pr *ParallelRenderer) RenderMultiple(ctx context.Context, requests []RenderRequest) (map[string]string, error) {
	if len(requests) == 0 {
		return make(map[string]string), nil
	}

	// Create channels for results and errors
	resultChan := make(chan RenderResult, len(requests))
	
	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, pr.maxConcurrency)
	
	// Use a wait group to track completion
	var wg sync.WaitGroup
	
	for _, req := range requests {
		wg.Add(1)
		go func(request RenderRequest) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()
			
			// Check context cancellation
			select {
			case <-ctx.Done():
				resultChan <- RenderResult{
					OutputKey: request.OutputKey,
					Error:     ctx.Err(),
				}
				return
			default:
			}
			
			// Render the template
			content, err := pr.engine.RenderTemplate(request.TemplateName, request.Data)
			resultChan <- RenderResult{
				OutputKey: request.OutputKey,
				Content:   content,
				Error:     err,
			}
		}(req)
	}
	
	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	results := make(map[string]string)
	var errors []error
	
	for result := range resultChan {
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("failed to render %s: %w", result.OutputKey, result.Error))
		} else {
			key := result.OutputKey
			if key == "" {
				key = fmt.Sprintf("result_%d", len(results))
			}
			results[key] = result.Content
		}
	}
	
	if len(errors) > 0 {
		return results, fmt.Errorf("failed to render %d templates: %v", len(errors), errors)
	}
	
	return results, nil
}

// RenderMultipleWithValidation renders multiple templates in parallel with validation
func (pr *ParallelRenderer) RenderMultipleWithValidation(ctx context.Context, requests []RenderRequest) (map[string]string, error) {
	if len(requests) == 0 {
		return make(map[string]string), nil
	}

	// Validate all templates first (sequentially for simplicity)
	for _, req := range requests {
		if err := pr.engine.ValidateTemplate(req.TemplateName); err != nil {
			return nil, fmt.Errorf("template validation failed for %s: %w", req.TemplateName, err)
		}
		
		if err := pr.engine.ValidateTemplateData(req.TemplateName, req.Data); err != nil {
			return nil, fmt.Errorf("template data validation failed for %s: %w", req.TemplateName, err)
		}
	}
	
	// Render in parallel
	return pr.RenderMultiple(ctx, requests)
}

// RenderBatch renders a batch of templates with the same data in parallel
func (pr *ParallelRenderer) RenderBatch(ctx context.Context, templateNames []string, data interface{}) (map[string]string, error) {
	if len(templateNames) == 0 {
		return make(map[string]string), nil
	}

	// Convert to render requests
	requests := make([]RenderRequest, len(templateNames))
	for i, name := range templateNames {
		requests[i] = RenderRequest{
			TemplateName: name,
			Data:         data,
			OutputKey:    name,
		}
	}
	
	return pr.RenderMultiple(ctx, requests)
}

// SetMaxConcurrency sets the maximum number of concurrent rendering operations
func (pr *ParallelRenderer) SetMaxConcurrency(maxConcurrency int) {
	if maxConcurrency > 0 {
		pr.maxConcurrency = maxConcurrency
	}
}

// GetMaxConcurrency returns the current maximum concurrency setting
func (pr *ParallelRenderer) GetMaxConcurrency() int {
	return pr.maxConcurrency
}
