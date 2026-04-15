package api

import (
	"errors"
	"net/http"
	"strconv"
	"travel/internal/db"
	"travel/internal/spaces"
	"travel/internal/sync"
	"travel/internal/upload"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PostSpacesRequest struct {
	SpaceID string `json:"space_id" binding:"required"`
	Name    string `json:"name" binding:"required"`
}

type PostSyncRequest struct {
	LastPulledAt *int64                           `json:"last_pulled_at" binding:"required"`
	Changes      map[string]sync.SyncChangeBucket `json:"changes" binding:"required"`
}

func HttpHello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "hello http",
	})
}

func HttpPostSpaces(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id header is required"})
		return
	}

	var req PostSpacesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := db.WithTx(func(tx *gorm.DB) error {
		return spaces.EnsureBinding(tx, userID, req.SpaceID, req.Name)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id and X-Space-Id headers are required"})
		return
	}

	rawLastPulledAt := c.Query("last_pulled_at")
	if rawLastPulledAt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "last_pulled_at query is required"})
		return
	}
	parsed, err := strconv.ParseInt(rawLastPulledAt, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid last_pulled_at"})
		return
	}
	lastPulledAt := sync.NormalizeTSMillis(parsed)

	changes, err := sync.BuildPullChangesForUser(userID, spaceID, lastPulledAt)
	if err != nil {
		if errors.Is(err, spaces.ErrUserNotInSpace) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id and X-Space-Id headers are required"})
		return
	}

	var req PostSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.LastPulledAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "last_pulled_at is required"})
		return
	}
	if req.Changes == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "changes is required"})
		return
	}
	lastPulledAt := sync.NormalizeTSMillis(*req.LastPulledAt)

	err := db.WithTx(func(tx *gorm.DB) error {
		return sync.ApplySyncChangesForUser(tx, userID, spaceID, lastPulledAt, req.Changes)
	})
	if err != nil {
		if errors.Is(err, spaces.ErrUserNotInSpace) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id and X-Space-Id headers are required"})
		return
	}

	if err := spaces.EnsureUserInSpace(userID, spaceID); err != nil {
		if errors.Is(err, spaces.ErrUserNotInSpace) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
