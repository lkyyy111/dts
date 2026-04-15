package sync

import (
	"gorm.io/gorm"
	"travel/internal/spaces"
)

func BuildPullChangesForUser(userID, headerSpaceID string, lastPulledAt int64) (map[string]PullChangeBucket, error) {
	if err := spaces.EnsureUserInSpace(userID, headerSpaceID); err != nil {
		return nil, err
	}
	return BuildPullChanges(headerSpaceID, lastPulledAt)
}

func ApplySyncChangesForUser(tx *gorm.DB, userID, headerSpaceID string, lastPulledAt int64, changes map[string]SyncChangeBucket) error {
	if err := spaces.EnsureUserInSpaceTx(tx, userID, headerSpaceID); err != nil {
		return err
	}
	// headerSpaceID 只用于合法性校验，push 仍然处理完整全局 changes。
	return ApplySyncChanges(tx, changes, lastPulledAt)
}
