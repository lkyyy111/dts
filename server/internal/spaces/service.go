package spaces

import (
	"errors"
	"fmt"
	"travel/internal/db"

	"gorm.io/gorm"
)

// EnsureBinding 建立或刷新当前用户与空间的关系（POST /api/v1/spaces）；核心关系表仅维护 id / 展示字段。
func EnsureBinding(tx *gorm.DB, userID, spaceID, spaceName string) error {
	if spaceName == "" {
		spaceName = "Untitled Space"
	}

	user := db.User{}
	err := tx.Where("id = ?", userID).First(&user).Error
	switch {
	case err == nil:
		if user.Nickname == "" {
			user.Nickname = "user-" + userID
			if err := tx.Model(&user).Update("nickname", user.Nickname).Error; err != nil {
				return err
			}
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		user = db.User{
			ID:       userID,
			Nickname: "user-" + userID,
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
		space.Name = spaceName
		if err := tx.Model(&space).Update("name", space.Name).Error; err != nil {
			return err
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		space = db.Space{
			ID:   spaceID,
			Name: spaceName,
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
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		member = db.SpaceMember{
			ID:      memberID,
			SpaceID: spaceID,
			UserID:  userID,
		}
		return tx.Create(&member).Error
	default:
		return err
	}
}
