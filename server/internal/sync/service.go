package sync

import (
	"encoding/json"
	"fmt"
)

// ParsePostSyncRequest 解析 POST /sync 的 body：支持顶层即 changes map，或 legacy 的 { last_pulled_at, changes }。
func ParsePostSyncRequest(rawBody []byte, queryLastPulled int64) (map[string]SyncChangeBucket, int64, error) {
	var rawEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &rawEnvelope); err != nil {
		return nil, 0, fmt.Errorf("invalid json body: %w", err)
	}

	changes := map[string]SyncChangeBucket{}
	_, hasLegacyChanges := rawEnvelope["changes"]
	lastPulledAt := queryLastPulled

	if hasLegacyChanges {
		var legacy LegacyPostSyncRequest
		if err := json.Unmarshal(rawBody, &legacy); err != nil {
			return nil, 0, fmt.Errorf("invalid legacy sync body: %w", err)
		}
		changes = NormalizeChangeMap(legacy.Changes)
		if lastPulledAt == 0 {
			lastPulledAt = legacy.LastPulledAt
		}
	} else {
		var directChanges map[string]SyncChangeBucket
		if err := json.Unmarshal(rawBody, &directChanges); err != nil {
			return nil, 0, fmt.Errorf("invalid changes body: %w", err)
		}
		changes = NormalizeChangeMap(directChanges)
	}

	if len(changes) == 0 {
		return nil, 0, fmt.Errorf("changes cannot be empty")
	}
	lastPulledAt = NormalizeTSMillis(lastPulledAt)
	return changes, lastPulledAt, nil
}
