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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

func main() {
	version := flag.String("version", "2.0", "Schema version to generate (only 2.0 is supported)")
	output := flag.String("output", "schema/cluster.schema.json", "Output file path")
	flag.Parse()

	// Validate version
	if *version != "2.0" && *version != "v2" && *version != "v2.0" {
		fmt.Fprintf(os.Stderr, "Error: Only v2.0 schema generation is supported in v2.0.0\n")
		os.Exit(1)
	}

	// Create schema generator
	generator := config.NewSchemaGenerator()

	// Generate schema
	schema, err := generator.Generate(*version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schema: %v\n", err)
		os.Exit(1)
	}

	// Validate schema output
	if err := config.ValidateSchemaOutput(schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error validating schema: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	if err := generator.WriteToFile(schema, *output); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing schema file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated schema v2.0 at %s\n", *output)
}
