package sync

import (
	"fmt"
	"travel/internal/db"

	"gorm.io/gorm"
)

func BuildPullChanges(spaceID string, lastPulledAt int64) (map[string]PullChangeBucket, error) {
	changes := map[string]PullChangeBucket{}
	var err error

	if changes["users"], err = pullCoreRelationUsers(spaceID); err != nil {
		return nil, fmt.Errorf("pull users failed: %w", err)
	}
	if changes["spaces"], err = pullCoreRelationSpaces(spaceID); err != nil {
		return nil, fmt.Errorf("pull spaces failed: %w", err)
	}
	if changes["space_members"], err = pullCoreRelationSpaceMembers(spaceID); err != nil {
		return nil, fmt.Errorf("pull space_members failed: %w", err)
	}
	if changes["photos"], err = pullPhotos(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull photos failed: %w", err)
	}
	if changes["expenses"], err = pullExpenses(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull expenses failed: %w", err)
	}
	if changes["posts"], err = pullPosts(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull posts failed: %w", err)
	}
	if changes["comments"], err = pullComments(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull comments failed: %w", err)
	}

	return changes, nil
}

// pullCoreRelationUsers 当前空间成员对应的 users，全部放入 created（不按 last_pulled_at 分类）。
func pullCoreRelationUsers(spaceID string) (PullChangeBucket, error) {
	var rows []db.User
	err := db.DB.Model(&db.User{}).
		Joins("JOIN space_members ON space_members.user_id = users.id").
		Where("space_members.space_id = ?", spaceID).
		Find(&rows).Error
	if err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapUsersForPull(rows),
		Updated: []any{},
		Deleted: []string{},
	}, nil
}

func pullCoreRelationSpaces(spaceID string) (PullChangeBucket, error) {
	var rows []db.Space
	if err := db.DB.Where("id = ?", spaceID).Find(&rows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapSpacesForPull(rows),
		Updated: []any{},
		Deleted: []string{},
	}, nil
}

func pullCoreRelationSpaceMembers(spaceID string) (PullChangeBucket, error) {
	var rows []db.SpaceMember
	if err := db.DB.Where("space_id = ?", spaceID).Find(&rows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapSpaceMembersForPull(rows),
		Updated: []any{},
		Deleted: []string{},
	}, nil
}

func pullPhotos(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.Photo
	var updatedRows []db.Photo
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.Photo{}).Session(&gorm.Session{})
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created_at > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created_at <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at > ?", spaceID, lastPulledAt).Pluck("id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapPhotosForPull(createdRows),
		Updated: mapPhotosForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}

func pullExpenses(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.Expense
	var updatedRows []db.Expense
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.Expense{}).Session(&gorm.Session{})
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created_at > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created_at <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at > ?", spaceID, lastPulledAt).Pluck("id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapExpensesForPull(createdRows),
		Updated: mapExpensesForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}

func pullPosts(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.Post
	var updatedRows []db.Post
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.Post{}).Session(&gorm.Session{})
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created_at > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created_at <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at > ?", spaceID, lastPulledAt).Pluck("id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapPostsForPull(createdRows),
		Updated: mapPostsForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}

func pullComments(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.Comment
	var updatedRows []db.Comment
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.Comment{}).Session(&gorm.Session{})
	}
	if err := baseQuery().
		Where("comments.space_id = ? AND comments.deleted_at = 0 AND comments.last_modified > ? AND comments.server_created_at > ?", spaceID, lastPulledAt, lastPulledAt).
		Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().
		Where("comments.space_id = ? AND comments.deleted_at = 0 AND comments.last_modified > ? AND comments.server_created_at <= ?", spaceID, lastPulledAt, lastPulledAt).
		Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().
		Where("comments.space_id = ? AND comments.deleted_at > ?", spaceID, lastPulledAt).
		Pluck("comments.id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapCommentsForPull(createdRows),
		Updated: mapCommentsForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}
