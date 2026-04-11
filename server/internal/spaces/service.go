package spaces

import (
	"errors"
	"fmt"
	"travel/internal/db"

	"gorm.io/gorm"
)

// EnsureBinding 建立或刷新当前用户与空间的关系（POST /api/v1/spaces）；不写客户端视角的 created_at/updated_at。
func EnsureBinding(tx *gorm.DB, userID, spaceID, spaceName string, ts int64) error {
	if spaceName == "" {
		spaceName = "Untitled Space"
	}

	user := db.User{}
	err := tx.Where("id = ?", userID).First(&user).Error
	switch {
	case err == nil:
		if ts >= user.LastModified {
			if user.Nickname == "" {
				user.Nickname = "user-" + userID
			}
			user.LastModified = ts
			if user.ServerCreatedAt == 0 {
				user.ServerCreatedAt = ts
			}
			user.DeletedAt = 0
			if err := tx.Save(&user).Error; err != nil {
				return err
			}
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		user = db.User{
			ID:              userID,
			Nickname:        "user-" + userID,
			DeletedAt:       0,
			LastModified:    ts,
			ServerCreatedAt: ts,
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
	default:
		return err
	}

	space := db.Space{}
	err = tx.Where("id = ?", spaceID).First(&space).Error
	switch {
	case err == nil:
		if ts >= space.LastModified {
			space.Name = spaceName
			space.LastModified = ts
			if space.ServerCreatedAt == 0 {
				space.ServerCreatedAt = ts
			}
			space.DeletedAt = 0
			if err := tx.Save(&space).Error; err != nil {
				return err
			}
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		space = db.Space{
			ID:              spaceID,
			Name:            spaceName,
			DeletedAt:       0,
			LastModified:    ts,
			ServerCreatedAt: ts,
		}
		if err := tx.Create(&space).Error; err != nil {
			return err
		}
	default:
		return err
	}

	memberID := fmt.Sprintf("%s_%s", spaceID, userID)
	member := db.SpaceMember{}
	err = tx.Where("id = ?", memberID).First(&member).Error
	switch {
	case err == nil:
		if ts >= member.LastModified {
			member.SpaceID = spaceID
			member.UserID = userID
			member.LastModified = ts
			if member.ServerCreatedAt == 0 {
				member.ServerCreatedAt = ts
			}
			member.DeletedAt = 0
			if err := tx.Save(&member).Error; err != nil {
				return err
			}
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		member = db.SpaceMember{
			ID:              memberID,
			SpaceID:         spaceID,
			UserID:          userID,
			DeletedAt:       0,
			LastModified:    ts,
			ServerCreatedAt: ts,
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
	default:
		return err
	}
	return nil
}
