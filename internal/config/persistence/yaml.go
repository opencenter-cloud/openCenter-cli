package persistence

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func MarshalYAML[T any](value *T) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("value cannot be nil")
	}
	return yaml.Marshal(value)
}

func UnmarshalYAML[T any](data []byte) (*T, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("value cannot be empty")
	}

	var value T
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return &value, nil
}
