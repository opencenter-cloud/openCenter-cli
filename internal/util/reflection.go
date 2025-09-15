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

package util

import (
	"reflect"
	"strings"
)

// FindField finds a struct field by its 'yaml' or 'json' tag.
//
// It takes a reflect.Value of a struct and the name of the tag to find.
// It returns the reflect.Value of the field if found, otherwise it returns
// an invalid reflect.Value.
//
// Inputs:
//   - v: The reflect.Value of the struct to search.
//   - name: The name of the yaml or json tag to find.
//
// Outputs:
//   - reflect.Value: The value of the found field, or an invalid value if not found.
func FindField(v reflect.Value, name string) reflect.Value {
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		yamlTag := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if yamlTag == name {
			return v.Field(i)
		}
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag == name {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}
