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

// Package errors provides structured error handling utilities with actionable suggestions.
//
// This package extracts common error handling patterns to provide reusable, testable
// components for creating, wrapping, aggregating, and formatting errors with rich
// context and user-friendly suggestions.
//
// Key interfaces:
//   - ErrorHandler: Handles and formats errors with suggestions
//   - ErrorWrapper: Wraps errors with additional context
//   - ErrorAggregator: Collects and manages multiple errors
//
// Key types:
//   - StructuredError: Rich error type with categorization, context, and suggestions
//   - ValidationResult: Result of validation operations with errors and warnings
//   - ErrorCollection: Collection of multiple errors
//
// The package supports error categorization, context preservation, suggestion
// generation, and specialized aggregators for validation and multi-field scenarios.
package errors
