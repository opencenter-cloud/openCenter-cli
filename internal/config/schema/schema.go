package schema

import "encoding/json"

const Version = "2.0"

func MarshalDocument(document map[string]any, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(document, "", "  ")
	}
	return json.Marshal(document)
}
