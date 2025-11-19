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

// Package paths provides path resolution and management utilities with organization support.
//
// This package extracts common path operations to provide reusable, testable components
// for resolving cluster paths, managing directory structures, and handling migrations
// from legacy flat structures to organization-based hierarchies.
//
// Key interfaces:
//   - PathResolver: Resolves paths with organization support
//   - MigrationManager: Handles structure migrations
//   - PathValidator: Validates paths and names
//   - DirectoryManager: Creates and manages directories
//   - PathExpander: Expands environment variables and user paths
//
// The package supports both legacy flat directory structures and new organization-based
// hierarchies, providing migration utilities to transition between them while maintaining
// backward compatibility.
package paths
