package build

import "encoding/json"

func encodeSnapshot(v map[string]any) ([]byte, error) {
	return json.Marshal(v)
}
