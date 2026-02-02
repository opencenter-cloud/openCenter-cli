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

package config

import (
	"fmt"
	"os"
	"sync"
)

var (
	// deprecationWarnings tracks which deprecation warnings have been shown
	deprecationWarnings = make(map[string]bool)
	deprecationMutex    sync.Mutex
)

// logDeprecationWarning logs a deprecation warning once per function.
// It uses stderr to avoid interfering with command output.
func logDeprecationWarning(functionName, replacement, removalVersion string) {
	deprecationMutex.Lock()
	defer deprecationMutex.Unlock()

	// Only show each warning once per execution
	if deprecationWarnings[functionName] {
		return
	}
	deprecationWarnings[functionName] = true

	// Check if deprecation warnings are disabled
	if os.Getenv("OPENCENTER_DISABLE_DEPRECATION_WARNINGS") == "true" {
		return
	}

	warning := fmt.Sprintf(
		"DEPRECATION WARNING: %s is deprecated and will be removed in %s.\n"+
			"  Please use %s instead.\n"+
			"  Set OPENCENTER_DISABLE_DEPRECATION_WARNINGS=true to suppress this warning.\n",
		functionName, removalVersion, replacement,
	)

	fmt.Fprint(os.Stderr, warning)
}
