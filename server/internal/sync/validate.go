package sync

import (
	"fmt"
	"strings"
)

func IsMillisTS(ts int64) bool {
	if ts == 0 {
		return true
	}
	return ts >= 1000000000000
}

func IsValidULID(v string) bool {
	if len(v) != 26 {
		return false
	}
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	upper := strings.ToUpper(v)
	for i := 0; i < len(upper); i++ {
		if !strings.ContainsRune(alphabet, rune(upper[i])) {
			return false
		}
	}
	return true
}

func NormalizeSpaceMemberID(item SyncSpaceMember) (SyncSpaceMember, error) {
	if item.SpaceID == "" || item.UserID == "" {
		return item, fmt.Errorf("space_id or user_id is empty")
	}
	if !IsValidULID(item.SpaceID) {
		return item, fmt.Errorf("invalid ULID for space_members.space_id: %s", item.SpaceID)
	}
	if !IsValidULID(item.UserID) {
		return item, fmt.Errorf("invalid ULID for space_members.user_id: %s", item.UserID)
	}
	item.ID = item.SpaceID + "_" + item.UserID
	return item, nil
}
