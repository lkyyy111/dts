package api

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"travel/internal/db"
	"travel/internal/spaces"
	"travel/internal/sync"
	"travel/internal/upload"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func HttpHello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "hello http",
	})
}

func HttpPostSpaces(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-Id header is required"})
		return
	}

	var req struct {
		SpaceID   string `json:"space_id" binding:"required"`
		SpaceName string `json:"space_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := db.WithTx(func(tx *gorm.DB) error {
		return spaces.EnsureBinding(tx, userID, req.SpaceID, req.SpaceName)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"space_id": req.SpaceID,
		"user_id":  userID,
	})
}

func HttpGetSync(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	spaceID := c.GetHeader("X-Space-Id")
	if userID == "" || spaceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-Id and X-Space-Id headers are required"})
		return
	}

	lastPulledAt := int64(0)
	rawLastPulledAt := c.Query("last_pulled_at")
	if rawLastPulledAt == "" {
		rawLastPulledAt = c.DefaultQuery("last_synced_at", "0")
	}
	if rawLastPulledAt != "" {
		parsed, err := strconv.ParseInt(rawLastPulledAt, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid last_pulled_at"})
			return
		}
		lastPulledAt = parsed
	}
	lastPulledAt = sync.NormalizeTSMillis(lastPulledAt)
	_ = c.Query("schema_version")
	_ = c.Query("migration")

	changes, err := sync.BuildPullChanges(spaceID, lastPulledAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sync.WatermelonPullResponse{
		Changes:   changes,
		Timestamp: sync.NowMillis(),
	})
}

func HttpPostSync(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	spaceID := c.GetHeader("X-Space-Id")
	if userID == "" || spaceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-Id and X-Space-Id headers are required"})
		return
	}

	lastPulledAt := int64(0)
	if raw := c.Query("last_pulled_at"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid last_pulled_at"})
			return
		}
		lastPulledAt = parsed
	}

	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(rawBody) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "changes cannot be empty"})
		return
	}

	changes, lastPulledAt, err := sync.ParsePostSyncRequest(rawBody, lastPulledAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = db.WithTx(func(tx *gorm.DB) error {
		return sync.ApplySyncChanges(tx, changes, lastPulledAt)
	})
	if err != nil {
		var conflictErr sync.ConflictError
		if errors.As(err, &conflictErr) {
			c.JSON(http.StatusConflict, gin.H{"error": conflictErr.Error()})
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrInvalidData) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func HttpPostPhotos(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	spaceID := c.GetHeader("X-Space-Id")
	if userID == "" || spaceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-Id and X-Space-Id headers are required"})
		return
	}

	remoteURL, status, err := upload.SavePhotoFromForm(c)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(status, gin.H{"remote_url": remoteURL})
}

func HttpPostAvatars(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	spaceID := c.GetHeader("X-Space-Id")
	if userID == "" || spaceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-Id and X-Space-Id headers are required"})
		return
	}

	remoteURL, status, err := upload.SaveAvatarFromForm(c)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(status, gin.H{"remote_url": remoteURL})
}
