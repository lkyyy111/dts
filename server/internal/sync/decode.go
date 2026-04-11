package sync

import (
	"encoding/json"
)

func DecodeBucket[T any](items []json.RawMessage) ([]T, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make([]T, 0, len(items))
	for _, item := range items {
		var decoded T
		if err := json.Unmarshal(item, &decoded); err != nil {
			return nil, err
		}
		out = append(out, decoded)
	}
	return out, nil
}
