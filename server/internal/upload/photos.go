package upload

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"travel/internal/config"
	"travel/internal/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SavePhotoFromForm 保存上传图片并在提供 photo_id 时写入/更新 photos 元数据。
func SavePhotoFromForm(c *gin.Context) (remoteURL string, httpStatus int, err error) {
	file, err := c.FormFile("file")
	if err != nil {
		return "", http.StatusBadRequest, fmt.Errorf("接受文件失败: %w", err)
	}

	saveDir := config.GlobalConfig.SFHD
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("服务器创建目录失败: %w", err)
	}

	ext := filepath.Ext(file.Filename)
	newFileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(saveDir, newFileName)
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("保存文件到服务器失败: %w", err)
	}

	host := config.GlobalConfig.Host
	port := config.GlobalConfig.Port
	fileURL := fmt.Sprintf("%s:%d/photos/%s", host, port, newFileName)

	photoID := c.PostForm("photo_id")
	spaceID := c.PostForm("space_id")
	uploaderID := c.PostForm("uploader_id")
	postID := c.DefaultPostForm("post_id", "")
	shotedAtRaw := c.DefaultPostForm("shoted_at", "0")
	shotedAt, _ := strconv.ParseInt(shotedAtRaw, 10, 64)
	ts := time.Now().UnixMilli()

	if photoID != "" {
		_ = db.WithTx(func(tx *gorm.DB) error {
			existing := db.Photo{}
			err := tx.Where("id = ?", photoID).First(&existing).Error
			switch {
			case err == nil:
				if ts >= existing.UpdatedAt {
					existing.SpaceID = pickString(spaceID, existing.SpaceID)
					existing.UploaderID = pickString(uploaderID, existing.UploaderID)
					existing.PostID = pickString(postID, existing.PostID)
					if shotedAt > 0 {
						existing.ShotedAt = shotedAt
					}
					existing.RemoteURL = fileURL
					existing.UpdatedAt = ts
					existing.DeletedAt = 0
					existing.LastModified = ts
					if existing.ServerCreatedAt == 0 {
						existing.ServerCreatedAt = ts
					}
					return tx.Save(&existing).Error
				}
				return nil
			case errors.Is(err, gorm.ErrRecordNotFound):
				record := db.Photo{
					ID:              photoID,
					SpaceID:         spaceID,
					UploaderID:      uploaderID,
					RemoteURL:       fileURL,
					PostID:          postID,
					ShotedAt:        shotedAt,
					CreatedAt:       ts,
					UpdatedAt:       ts,
					DeletedAt:       0,
					LastModified:    ts,
					ServerCreatedAt: ts,
				}
				return tx.Create(&record).Error
			default:
				return err
			}
		})
	}

	return fileURL, http.StatusOK, nil
}

func pickString(v string, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
