package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"strconv"
	"time"
	"travel/internal/config"
	"travel/internal/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func HttpHello(c *gin.Context) {
	// Function name should be Capital uppercase
	// for other packages to call
	c.JSON(http.StatusOK, gin.H{
		"message": "hello http",
	})
}

type PostSpacesRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	SpaceID  string `json:"space_id" binding:"required"`
	SpaceName string `json:"space_name"`
}

type RecordChanges[T any] struct {
	Upserts []T      `json:"upserts"`
	Deletes []string `json:"deletes"`
}

type SyncChanges struct {
	Users        RecordChanges[db.User]        `json:"users"`
	Spaces       RecordChanges[db.Space]       `json:"spaces"`
	SpaceMembers RecordChanges[db.SpaceMember] `json:"space_members"`
	Posts        RecordChanges[db.Post]        `json:"posts"`
	Photos       RecordChanges[db.Photo]       `json:"photos"`
	Comments     RecordChanges[db.Comment]     `json:"comments"`
	Expenses     RecordChanges[db.Expense]     `json:"expenses"`
}

type LegacyPostSyncRequest struct {
	LastPulledAt int64                        `json:"last_pulled_at"`
	Changes      map[string]SyncChangeBucket `json:"changes"`
}

type SyncChangeBucket struct {
	Created []json.RawMessage `json:"created"`
	Updated []json.RawMessage `json:"updated"`
	Deleted []string          `json:"deleted"`
}

type SyncUser struct {
	ID              string `json:"id"`
	Nickname        string `json:"nickname"`
	AvatarLocalURI  string `json:"avatar_local_uri"`
	AvatarRemoteURL string `json:"avatar_remote_url"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	DeletedAt       int64  `json:"deleted_at"`
}

type SyncSpace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt int64  `json:"deleted_at"`
}

type SyncSpaceMember struct {
	ID        string `json:"id"`
	SpaceID   string `json:"space_id"`
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt int64  `json:"deleted_at"`
}

type SyncPhoto struct {
	ID         string `json:"id"`
	SpaceID    string `json:"space_id"`
	UploaderID string `json:"uploader_id"`
	RemoteURL  string `json:"remote_url"`
	LocalURI   string `json:"local_uri"`
	PostID     string `json:"post_id"`
	ShotedAt   int64  `json:"shoted_at"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
	DeletedAt  int64  `json:"deleted_at"`
}

type SyncExpense struct {
	ID          string  `json:"id"`
	SpaceID     string  `json:"space_id"`
	PayerID     string  `json:"payer_id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
	DeletedAt   int64   `json:"deleted_at"`
}

type SyncPost struct {
	ID        string `json:"id"`
	SpaceID   string `json:"space_id"`
	PosterID  string `json:"poster_id"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt int64  `json:"deleted_at"`
}

type SyncComment struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	CommenterID string `json:"commenter_id"`
	PostID      string `json:"post_id"`
	CommentedAt int64  `json:"commented_at"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	DeletedAt   int64  `json:"deleted_at"`
}

type PullChangeBucket struct {
	Created []any    `json:"created"`
	Updated []any    `json:"updated"`
	Deleted []string `json:"deleted"`
}

type WatermelonPullResponse struct {
	Changes   map[string]PullChangeBucket `json:"changes"`
	Timestamp int64                       `json:"timestamp"`
}

type pushMode int

const (
	pushModeCreated pushMode = iota
	pushModeUpdated
)

type syncConflictError struct {
	table string
	id    string
}

func (e syncConflictError) Error() string {
	return fmt.Sprintf("conflict on %s: id=%s modified after last_pulled_at", e.table, e.id)
}

type ProcessedBucket struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}

func normalizeTSMillis(ts int64) int64 {
	// Keep client-provided timestamp as-is.
	// Server-side fallback to current millis is handled at write points.
	return ts
}

func normalizeChangeKey(key string) string {
	switch key {
	case "users", "change_users":
		return "users"
	case "spaces", "change_spaces":
		return "spaces"
	case "space_members", "change_space_members", "change_space_menbers":
		return "space_members"
	case "photos", "change_photos":
		return "photos"
	case "expenses", "change_expenses":
		return "expenses"
	case "posts", "change_posts":
		return "posts"
	case "comments", "change_comments":
		return "comments"
	default:
		return ""
	}
}

func mergeBuckets(dst SyncChangeBucket, src SyncChangeBucket) SyncChangeBucket {
	dst.Created = append(dst.Created, src.Created...)
	dst.Updated = append(dst.Updated, src.Updated...)
	dst.Deleted = append(dst.Deleted, src.Deleted...)
	return dst
}

func normalizeChangeMap(changes map[string]SyncChangeBucket) map[string]SyncChangeBucket {
	out := map[string]SyncChangeBucket{}
	for rawKey, bucket := range changes {
		key := normalizeChangeKey(rawKey)
		if key == "" {
			continue
		}
		out[key] = mergeBuckets(out[key], bucket)
	}
	return out
}

func isMillisTS(ts int64) bool {
	if ts == 0 {
		return true
	}
	return ts >= 1000000000000
}

func HttpPostSpaces(c *gin.Context) {
	var req PostSpacesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ts := nowMillis()
	if req.SpaceName == "" {
		req.SpaceName = "Untitled Space"
	}

	err := db.WithTx(func(tx *gorm.DB) error {
		user := db.User{}
		err := tx.Where("id = ?", req.UserID).First(&user).Error
		switch {
		case err == nil:
			if ts >= user.UpdatedAt {
				if user.Nickname == "" {
					user.Nickname = "user-" + req.UserID
				}
				user.UpdatedAt = ts
				user.LastModified = ts
				if user.CreatedAt == 0 {
					user.CreatedAt = ts
				}
				if user.ServerCreated == 0 {
					user.ServerCreated = ts
				}
				user.DeletedAt = 0
				if err := tx.Save(&user).Error; err != nil {
					return err
				}
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			user = db.User{
				ID:              req.UserID,
				Nickname:        "user-" + req.UserID,
				AvatarRemoteURL: "",
				CreatedAt:       ts,
				UpdatedAt:       ts,
				DeletedAt:       0,
				LastModified:    ts,
				ServerCreated:   ts,
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		default:
			return err
		}

		space := db.Space{}
		err = tx.Where("id = ?", req.SpaceID).First(&space).Error
		switch {
		case err == nil:
			if ts >= space.UpdatedAt {
				space.Name = req.SpaceName
				space.UpdatedAt = ts
				space.LastModified = ts
				if space.CreatedAt == 0 {
					space.CreatedAt = ts
				}
				if space.ServerCreated == 0 {
					space.ServerCreated = ts
				}
				space.DeletedAt = 0
				if err := tx.Save(&space).Error; err != nil {
					return err
				}
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			space = db.Space{
				ID: req.SpaceID, Name: req.SpaceName, CreatedAt: ts, UpdatedAt: ts, DeletedAt: 0, LastModified: ts, ServerCreated: ts,
			}
			if err := tx.Create(&space).Error; err != nil {
				return err
			}
		default:
			return err
		}

		memberID := fmt.Sprintf("%s_%s", req.SpaceID, req.UserID)
		member := db.SpaceMember{}
		err = tx.Where("id = ?", memberID).First(&member).Error
		switch {
		case err == nil:
			if ts >= member.UpdatedAt {
				member.SpaceID = req.SpaceID
				member.UserID = req.UserID
				member.UpdatedAt = ts
				member.LastModified = ts
				if member.CreatedAt == 0 {
					member.CreatedAt = ts
				}
				if member.ServerCreated == 0 {
					member.ServerCreated = ts
				}
				member.DeletedAt = 0
				if err := tx.Save(&member).Error; err != nil {
					return err
				}
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			member = db.SpaceMember{
				ID: memberID, SpaceID: req.SpaceID, UserID: req.UserID, CreatedAt: ts, UpdatedAt: ts, DeletedAt: 0, LastModified: ts, ServerCreated: ts,
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		default:
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"space_id": req.SpaceID,
		"user_id":  req.UserID,
	})
}

type SyncResponse struct {
	SpaceID       string      `json:"space_id"`
	LastSyncedAt  int64       `json:"last_synced_at"`
	ServerTS      int64       `json:"server_ts"`
	Changes       SyncChanges `json:"changes"`
}

func HttpGetSync(c *gin.Context) {
	spaceID := c.Query("space_id")
	if spaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "space_id is required"})
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
	lastPulledAt = normalizeTSMillis(lastPulledAt)
	_ = c.Query("schema_version")
	_ = c.Query("migration")

	changes, err := buildPullChanges(spaceID, lastPulledAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, WatermelonPullResponse{
		Changes:   changes,
		Timestamp: nowMillis(),
	})
}

func softDeleteByIDs(tx *gorm.DB, table string, ids []string, ts int64) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Table(table).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"deleted_at":    ts,
			"updated_at":    ts,
			"last_modified": ts,
		}).Error
}

func HttpPostSync(c *gin.Context) {
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

	var rawEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &rawEnvelope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}

	changes := map[string]SyncChangeBucket{}
	_, hasLegacyChanges := rawEnvelope["changes"]
	if hasLegacyChanges {
		var legacy LegacyPostSyncRequest
		if err := json.Unmarshal(rawBody, &legacy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid legacy sync body"})
			return
		}
		changes = normalizeChangeMap(legacy.Changes)
		if lastPulledAt == 0 {
			lastPulledAt = legacy.LastPulledAt
		}
	} else {
		var directChanges map[string]SyncChangeBucket
		if err := json.Unmarshal(rawBody, &directChanges); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid changes body"})
			return
		}
		changes = normalizeChangeMap(directChanges)
	}

	if len(changes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "changes cannot be empty"})
		return
	}
	lastPulledAt = normalizeTSMillis(lastPulledAt)

	err = db.WithTx(func(tx *gorm.DB) error {
		if err := applySyncChanges(tx, changes, lastPulledAt); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var conflictErr syncConflictError
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

	_ = lastPulledAt
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func decodeBucket[T any](items []json.RawMessage) ([]T, error) {
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

func applySyncChanges(tx *gorm.DB, changes map[string]SyncChangeBucket, lastPulledAt int64) error {
	type groupHandler struct {
		key     string
		handler func(*gorm.DB, SyncChangeBucket, int64) error
	}
	handlers := []groupHandler{
		{"users", applyUserChanges},
		{"spaces", applySpaceChanges},
		{"space_members", applySpaceMemberChanges},
		{"photos", applyPhotoChanges},
		{"expenses", applyExpenseChanges},
		{"posts", applyPostChanges},
		{"comments", applyCommentChanges},
	}
	for _, h := range handlers {
		bucket, ok := changes[h.key]
		if !ok {
			continue
		}
		if err := h.handler(tx, bucket, lastPulledAt); err != nil {
			return err
		}
	}
	return nil
}

func applyUserChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := decodeBucket[SyncUser](bucket.Created)
	if err != nil {
		return fmt.Errorf("process change_users created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := decodeBucket[SyncUser](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process change_users updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if !isMillisTS(item.CreatedAt) || !isMillisTS(item.UpdatedAt) || !isMillisTS(item.DeletedAt) {
			return fmt.Errorf("process change_users upsert failed: %w", gorm.ErrInvalidData)
		}
		if err := upsertUser(tx, item, pushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_users upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if !isMillisTS(item.CreatedAt) || !isMillisTS(item.UpdatedAt) || !isMillisTS(item.DeletedAt) {
			return fmt.Errorf("process change_users upsert failed: %w", gorm.ErrInvalidData)
		}
		if err := upsertUser(tx, item, pushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_users upsert failed: %w", err)
		}
	}
	if err := softDeleteByIDs(tx, "users", bucket.Deleted, nowMillis()); err != nil {
		return fmt.Errorf("process change_users deleted failed: %w", err)
	}
	return nil
}

func applySpaceChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := decodeBucket[SyncSpace](bucket.Created)
	if err != nil {
		return fmt.Errorf("process change_spaces created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := decodeBucket[SyncSpace](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process change_spaces updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if !isMillisTS(item.CreatedAt) || !isMillisTS(item.UpdatedAt) || !isMillisTS(item.DeletedAt) {
			return fmt.Errorf("process change_spaces upsert failed: %w", gorm.ErrInvalidData)
		}
		if err := upsertSpace(tx, item, pushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_spaces upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if !isMillisTS(item.CreatedAt) || !isMillisTS(item.UpdatedAt) || !isMillisTS(item.DeletedAt) {
			return fmt.Errorf("process change_spaces upsert failed: %w", gorm.ErrInvalidData)
		}
		if err := upsertSpace(tx, item, pushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_spaces upsert failed: %w", err)
		}
	}
	if err := softDeleteByIDs(tx, "spaces", bucket.Deleted, nowMillis()); err != nil {
		return fmt.Errorf("process change_spaces deleted failed: %w", err)
	}
	return nil
}

func applySpaceMemberChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := decodeBucket[SyncSpaceMember](bucket.Created)
	if err != nil {
		return fmt.Errorf("process change_space_menbers created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := decodeBucket[SyncSpaceMember](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process change_space_menbers updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		normalized, err := normalizeSpaceMemberID(item)
		if err != nil {
			return fmt.Errorf("process change_space_menbers failed: %s: %w", err.Error(), gorm.ErrInvalidData)
		}
		if !isMillisTS(normalized.CreatedAt) || !isMillisTS(normalized.UpdatedAt) || !isMillisTS(normalized.DeletedAt) {
			return fmt.Errorf("process change_space_menbers upsert failed: %w", gorm.ErrInvalidData)
		}
		if err := upsertSpaceMember(tx, normalized, pushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_space_menbers upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		normalized, err := normalizeSpaceMemberID(item)
		if err != nil {
			return fmt.Errorf("process change_space_menbers failed: %s: %w", err.Error(), gorm.ErrInvalidData)
		}
		if !isMillisTS(normalized.CreatedAt) || !isMillisTS(normalized.UpdatedAt) || !isMillisTS(normalized.DeletedAt) {
			return fmt.Errorf("process change_space_menbers upsert failed: %w", gorm.ErrInvalidData)
		}
		if err := upsertSpaceMember(tx, normalized, pushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_space_menbers upsert failed: %w", err)
		}
	}
	if err := softDeleteByIDs(tx, "space_members", bucket.Deleted, nowMillis()); err != nil {
		return fmt.Errorf("process change_space_menbers deleted failed: %w", err)
	}
	return nil
}

func normalizeSpaceMemberID(item SyncSpaceMember) (SyncSpaceMember, error) {
	if item.SpaceID == "" || item.UserID == "" {
		return item, fmt.Errorf("space_id or user_id is empty")
	}
	if !isValidULID(item.SpaceID) {
		return item, fmt.Errorf("invalid ULID for space_members.space_id: %s", item.SpaceID)
	}
	if !isValidULID(item.UserID) {
		return item, fmt.Errorf("invalid ULID for space_members.user_id: %s", item.UserID)
	}
	item.ID = item.SpaceID + "_" + item.UserID
	return item, nil
}

func isValidULID(v string) bool {
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

func applyPhotoChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := decodeBucket[SyncPhoto](bucket.Created)
	if err != nil {
		return fmt.Errorf("process change_photos created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := decodeBucket[SyncPhoto](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process change_photos updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertPhoto(tx, item, pushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_photos upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertPhoto(tx, item, pushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_photos upsert failed: %w", err)
		}
	}
	if err := softDeleteByIDs(tx, "photos", bucket.Deleted, nowMillis()); err != nil {
		return fmt.Errorf("process change_photos deleted failed: %w", err)
	}
	return nil
}

func applyExpenseChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := decodeBucket[SyncExpense](bucket.Created)
	if err != nil {
		return fmt.Errorf("process change_expenses created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := decodeBucket[SyncExpense](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process change_expenses updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertExpense(tx, item, pushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_expenses upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertExpense(tx, item, pushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_expenses upsert failed: %w", err)
		}
	}
	if err := softDeleteByIDs(tx, "expenses", bucket.Deleted, nowMillis()); err != nil {
		return fmt.Errorf("process change_expenses deleted failed: %w", err)
	}
	return nil
}

func applyPostChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := decodeBucket[SyncPost](bucket.Created)
	if err != nil {
		return fmt.Errorf("process change_posts created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := decodeBucket[SyncPost](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process change_posts updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertPost(tx, item, pushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_posts upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertPost(tx, item, pushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_posts upsert failed: %w", err)
		}
	}
	if err := softDeleteByIDs(tx, "posts", bucket.Deleted, nowMillis()); err != nil {
		return fmt.Errorf("process change_posts deleted failed: %w", err)
	}
	return nil
}

func applyCommentChanges(tx *gorm.DB, bucket SyncChangeBucket, lastPulledAt int64) error {
	created, err := decodeBucket[SyncComment](bucket.Created)
	if err != nil {
		return fmt.Errorf("process change_comments created failed: %w", gorm.ErrInvalidData)
	}
	updated, err := decodeBucket[SyncComment](bucket.Updated)
	if err != nil {
		return fmt.Errorf("process change_comments updated failed: %w", gorm.ErrInvalidData)
	}
	for _, item := range created {
		if item.ID == "" {
			continue
		}
		if err := upsertComment(tx, item, pushModeCreated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_comments upsert failed: %w", err)
		}
	}
	for _, item := range updated {
		if item.ID == "" {
			continue
		}
		if err := upsertComment(tx, item, pushModeUpdated, lastPulledAt); err != nil {
			return fmt.Errorf("process change_comments upsert failed: %w", err)
		}
	}
	if err := softDeleteByIDs(tx, "comments", bucket.Deleted, nowMillis()); err != nil {
		return fmt.Errorf("process change_comments deleted failed: %w", err)
	}
	return nil
}

func upsertUser(tx *gorm.DB, item SyncUser, mode pushMode, lastPulledAt int64) error {
	var existing db.User
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := nowMillis()
	record := db.User{
		ID:              item.ID,
		Nickname:        item.Nickname,
		AvatarRemoteURL: item.AvatarRemoteURL,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		DeletedAt:       item.DeletedAt,
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == pushModeUpdated && existing.LastModified > lastPulledAt {
			return syncConflictError{table: "users", id: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"nickname":          record.Nickname,
			"avatar_remote_url": record.AvatarRemoteURL,
			"created_at":        record.CreatedAt,
			"updated_at":        record.UpdatedAt,
			"deleted_at":        record.DeletedAt,
			"last_modified":     ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreated = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertSpace(tx *gorm.DB, item SyncSpace, mode pushMode, lastPulledAt int64) error {
	var existing db.Space
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := nowMillis()
	record := db.Space{
		ID:        item.ID,
		Name:      item.Name,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
		DeletedAt: item.DeletedAt,
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == pushModeUpdated && existing.LastModified > lastPulledAt {
			return syncConflictError{table: "spaces", id: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"name":       record.Name,
			"created_at": record.CreatedAt,
			"updated_at": record.UpdatedAt,
			"deleted_at": record.DeletedAt,
			"last_modified": ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreated = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertSpaceMember(tx *gorm.DB, item SyncSpaceMember, mode pushMode, lastPulledAt int64) error {
	var existing db.SpaceMember
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := nowMillis()
	record := db.SpaceMember{
		ID:        item.ID,
		SpaceID:   item.SpaceID,
		UserID:    item.UserID,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
		DeletedAt: item.DeletedAt,
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == pushModeUpdated && existing.LastModified > lastPulledAt {
			return syncConflictError{table: "space_members", id: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":   record.SpaceID,
			"user_id":    record.UserID,
			"created_at": record.CreatedAt,
			"updated_at": record.UpdatedAt,
			"deleted_at": record.DeletedAt,
			"last_modified": ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreated = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertPhoto(tx *gorm.DB, item SyncPhoto, mode pushMode, lastPulledAt int64) error {
	var existing db.Photo
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := nowMillis()
	record := db.Photo{
		ID:         item.ID,
		SpaceID:    item.SpaceID,
		UploaderID: item.UploaderID,
		RemoteURL:  item.RemoteURL,
		PostID:     item.PostID,
		ShotedAt:   normalizeTSMillis(item.ShotedAt),
		CreatedAt:  normalizeTSMillis(item.CreatedAt),
		UpdatedAt:  normalizeTSMillis(item.UpdatedAt),
		DeletedAt:  normalizeTSMillis(item.DeletedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == pushModeUpdated && existing.LastModified > lastPulledAt {
			return syncConflictError{table: "photos", id: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":       record.SpaceID,
			"uploader_id":    record.UploaderID,
			"remote_url":     record.RemoteURL,
			"post_id":        record.PostID,
			"shoted_at":      record.ShotedAt,
			"created_at":     record.CreatedAt,
			"updated_at":     record.UpdatedAt,
			"deleted_at":     record.DeletedAt,
			"last_modified":  ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreated = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertExpense(tx *gorm.DB, item SyncExpense, mode pushMode, lastPulledAt int64) error {
	var existing db.Expense
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := nowMillis()
	record := db.Expense{
		ID:          item.ID,
		SpaceID:     item.SpaceID,
		PayerID:     item.PayerID,
		Amount:      item.Amount,
		Description: item.Description,
		CreatedAt:   normalizeTSMillis(item.CreatedAt),
		UpdatedAt:   normalizeTSMillis(item.UpdatedAt),
		DeletedAt:   normalizeTSMillis(item.DeletedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == pushModeUpdated && existing.LastModified > lastPulledAt {
			return syncConflictError{table: "expenses", id: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":       record.SpaceID,
			"payer_id":       record.PayerID,
			"amount":         record.Amount,
			"description":    record.Description,
			"created_at":     record.CreatedAt,
			"updated_at":     record.UpdatedAt,
			"deleted_at":     record.DeletedAt,
			"last_modified":  ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreated = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertPost(tx *gorm.DB, item SyncPost, mode pushMode, lastPulledAt int64) error {
	var existing db.Post
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := nowMillis()
	record := db.Post{
		ID:        item.ID,
		SpaceID:   item.SpaceID,
		PosterID:  item.PosterID,
		Content:   item.Content,
		CreatedAt: normalizeTSMillis(item.CreatedAt),
		UpdatedAt: normalizeTSMillis(item.UpdatedAt),
		DeletedAt: normalizeTSMillis(item.DeletedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == pushModeUpdated && existing.LastModified > lastPulledAt {
			return syncConflictError{table: "posts", id: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"space_id":   record.SpaceID,
			"poster_id":  record.PosterID,
			"content":    record.Content,
			"created_at": record.CreatedAt,
			"updated_at": record.UpdatedAt,
			"deleted_at": record.DeletedAt,
			"last_modified": ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreated = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func upsertComment(tx *gorm.DB, item SyncComment, mode pushMode, lastPulledAt int64) error {
	var existing db.Comment
	err := tx.Where("id = ?", item.ID).First(&existing).Error
	ts := nowMillis()
	record := db.Comment{
		ID:          item.ID,
		Content:     item.Content,
		CommenterID: item.CommenterID,
		PostID:      item.PostID,
		CommentedAt: normalizeTSMillis(item.CommentedAt),
		CreatedAt:   normalizeTSMillis(item.CreatedAt),
		UpdatedAt:   normalizeTSMillis(item.UpdatedAt),
		DeletedAt:   normalizeTSMillis(item.DeletedAt),
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = ts
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = ts
	}
	switch {
	case err == nil:
		if mode == pushModeUpdated && existing.LastModified > lastPulledAt {
			return syncConflictError{table: "comments", id: item.ID}
		}
		return tx.Model(&existing).UpdateColumns(map[string]any{
			"content":        record.Content,
			"commenter_id":   record.CommenterID,
			"post_id":        record.PostID,
			"commented_at":   record.CommentedAt,
			"created_at":     record.CreatedAt,
			"updated_at":     record.UpdatedAt,
			"deleted_at":     record.DeletedAt,
			"last_modified":  ts,
		}).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		record.ServerCreated = ts
		record.LastModified = ts
		return tx.Create(&record).Error
	default:
		return err
	}
}

func buildPullChanges(spaceID string, lastPulledAt int64) (map[string]PullChangeBucket, error) {
	changes := map[string]PullChangeBucket{}
	var err error

	if changes["users"], err = pullUsers(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull change_users failed: %w", err)
	}
	if changes["spaces"], err = pullSpaces(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull change_spaces failed: %w", err)
	}
	if changes["space_members"], err = pullSpaceMembers(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull change_space_menbers failed: %w", err)
	}
	if changes["photos"], err = pullPhotos(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull change_photos failed: %w", err)
	}
	if changes["expenses"], err = pullExpenses(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull change_expenses failed: %w", err)
	}
	if changes["posts"], err = pullPosts(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull change_posts failed: %w", err)
	}
	if changes["comments"], err = pullComments(spaceID, lastPulledAt); err != nil {
		return nil, fmt.Errorf("pull change_comments failed: %w", err)
	}

	return changes, nil
}

func pullUsers(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.User
	var updatedRows []db.User
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.User{}).
			Joins("JOIN space_members ON space_members.user_id = users.id").
			Where("space_members.space_id = ?", spaceID).
			Session(&gorm.Session{})
	}
	if err := baseQuery().Where("users.deleted_at = 0 AND users.last_modified > ? AND users.server_created > ?", lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("users.deleted_at = 0 AND users.last_modified > ? AND users.server_created <= ?", lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("users.deleted_at > ?", lastPulledAt).Pluck("users.id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapUsersForPull(createdRows),
		Updated: mapUsersForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}

func pullSpaces(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.Space
	var updatedRows []db.Space
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.Space{}).Session(&gorm.Session{})
	}
	if err := baseQuery().Where("id = ? AND deleted_at = 0 AND last_modified > ? AND server_created > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("id = ? AND deleted_at = 0 AND last_modified > ? AND server_created <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("id = ? AND deleted_at > ?", spaceID, lastPulledAt).Pluck("id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapSpacesForPull(createdRows),
		Updated: mapSpacesForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}

func pullSpaceMembers(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.SpaceMember
	var updatedRows []db.SpaceMember
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.SpaceMember{}).Session(&gorm.Session{})
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at > ?", spaceID, lastPulledAt).Pluck("id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapSpaceMembersForPull(createdRows),
		Updated: mapSpaceMembersForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}

func pullPhotos(spaceID string, lastPulledAt int64) (PullChangeBucket, error) {
	var createdRows []db.Photo
	var updatedRows []db.Photo
	var deletedIDs []string
	baseQuery := func() *gorm.DB {
		return db.DB.Model(&db.Photo{}).Session(&gorm.Session{})
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
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
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
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
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created > ?", spaceID, lastPulledAt, lastPulledAt).Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().Where("space_id = ? AND deleted_at = 0 AND last_modified > ? AND server_created <= ?", spaceID, lastPulledAt, lastPulledAt).Find(&updatedRows).Error; err != nil {
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
		return db.DB.Model(&db.Comment{}).
			Joins("JOIN posts ON posts.id = comments.post_id").
			Session(&gorm.Session{})
	}
	if err := baseQuery().
		Where("posts.space_id = ? AND comments.deleted_at = 0 AND comments.last_modified > ? AND comments.server_created > ?", spaceID, lastPulledAt, lastPulledAt).
		Find(&createdRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().
		Where("posts.space_id = ? AND comments.deleted_at = 0 AND comments.last_modified > ? AND comments.server_created <= ?", spaceID, lastPulledAt, lastPulledAt).
		Find(&updatedRows).Error; err != nil {
		return PullChangeBucket{}, err
	}
	if err := baseQuery().
		Where("posts.space_id = ? AND comments.deleted_at > ?", spaceID, lastPulledAt).
		Pluck("comments.id", &deletedIDs).Error; err != nil {
		return PullChangeBucket{}, err
	}
	return PullChangeBucket{
		Created: mapCommentsForPull(createdRows),
		Updated: mapCommentsForPull(updatedRows),
		Deleted: deletedIDs,
	}, nil
}

func mapUsersForPull(rows []db.User) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":                r.ID,
			"nickname":          r.Nickname,
			"avatar_remote_url": r.AvatarRemoteURL,
			"avatar_local_uri":  "",
			"created_at":        normalizeTSMillis(r.CreatedAt),
			"updated_at":        normalizeTSMillis(r.UpdatedAt),
			"deleted_at":        normalizeTSMillis(r.DeletedAt),
		})
	}
	return out
}

func mapSpacesForPull(rows []db.Space) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":         r.ID,
			"name":       r.Name,
			"created_at": normalizeTSMillis(r.CreatedAt),
			"updated_at": normalizeTSMillis(r.UpdatedAt),
			"deleted_at": normalizeTSMillis(r.DeletedAt),
		})
	}
	return out
}

func mapSpaceMembersForPull(rows []db.SpaceMember) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":         r.SpaceID + "_" + r.UserID,
			"space_id":   r.SpaceID,
			"user_id":    r.UserID,
			"created_at": normalizeTSMillis(r.CreatedAt),
			"updated_at": normalizeTSMillis(r.UpdatedAt),
			"deleted_at": normalizeTSMillis(r.DeletedAt),
		})
	}
	return out
}

func mapPhotosForPull(rows []db.Photo) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":          r.ID,
			"space_id":    r.SpaceID,
			"uploader_id": r.UploaderID,
			"remote_url":  r.RemoteURL,
			"local_uri":   "",
			"post_id":     r.PostID,
			"shoted_at":   normalizeTSMillis(r.ShotedAt),
			"created_at":  normalizeTSMillis(r.CreatedAt),
			"updated_at":  normalizeTSMillis(r.UpdatedAt),
			"deleted_at":  normalizeTSMillis(r.DeletedAt),
		})
	}
	return out
}

func mapExpensesForPull(rows []db.Expense) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":          r.ID,
			"space_id":    r.SpaceID,
			"payer_id":    r.PayerID,
			"amount":      r.Amount,
			"description": r.Description,
			"created_at":  normalizeTSMillis(r.CreatedAt),
			"updated_at":  normalizeTSMillis(r.UpdatedAt),
			"deleted_at":  normalizeTSMillis(r.DeletedAt),
		})
	}
	return out
}

func mapPostsForPull(rows []db.Post) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":         r.ID,
			"space_id":   r.SpaceID,
			"poster_id":  r.PosterID,
			"content":    r.Content,
			"created_at": normalizeTSMillis(r.CreatedAt),
			"updated_at": normalizeTSMillis(r.UpdatedAt),
			"deleted_at": normalizeTSMillis(r.DeletedAt),
		})
	}
	return out
}

func mapCommentsForPull(rows []db.Comment) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":           r.ID,
			"content":      r.Content,
			"commenter_id": r.CommenterID,
			"post_id":      r.PostID,
			"commented_at": normalizeTSMillis(r.CommentedAt),
			"created_at":   normalizeTSMillis(r.CreatedAt),
			"updated_at":   normalizeTSMillis(r.UpdatedAt),
			"deleted_at":   normalizeTSMillis(r.DeletedAt),
		})
	}
	return out
}

// 处理`POST photos`请求
// TODO：未更新数据库，仅保存了文件
func HttpPostPhotos(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "接受文件失败: " + err.Error(),
		})
		return
	}
	
	saveDir := config.GlobalConfig.SFHD
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "服务器创建目录失败",
		})
		return
	}
	
	ext := filepath.Ext(file.Filename)
	newFileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

	savePath := filepath.Join(saveDir, newFileName)
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存文件到服务器失败",
		})
		return
	}

	// 读取配置
	host := config.GlobalConfig.Host
	port := config.GlobalConfig.Port

	fileUrl := fmt.Sprintf("%s:%d/photos/%s", host, port, newFileName)

	photoID := c.PostForm("photo_id")
	spaceID := c.PostForm("space_id")
	uploaderID := c.PostForm("uploader_id")
	postID := c.DefaultPostForm("post_id", "")
	shotedAtRaw := c.DefaultPostForm("shoted_at", "0")
	shotedAt, _ := strconv.ParseInt(shotedAtRaw, 10, 64)
	ts := nowMillis()

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
					existing.RemoteURL = fileUrl
					existing.UpdatedAt = ts
					existing.DeletedAt = 0
					existing.LastModified = ts
					if existing.ServerCreated == 0 {
						existing.ServerCreated = ts
					}
					return tx.Save(&existing).Error
				}
				return nil
			case errors.Is(err, gorm.ErrRecordNotFound):
				record := db.Photo{
					ID:         photoID,
					SpaceID:    spaceID,
					UploaderID: uploaderID,
					RemoteURL:  fileUrl,
					PostID:     postID,
					ShotedAt:   shotedAt,
					CreatedAt:  ts,
					UpdatedAt:  ts,
					DeletedAt:  0,
					LastModified: ts,
					ServerCreated: ts,
				}
				return tx.Create(&record).Error
			default:
				return err
			}
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"remote_url": fileUrl,
	})
}

func HttpPostAvatars(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少文件字段 file: " + err.Error(),
		})
		return
	}

	saveDir := "./photos"
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建目录失败: " + err.Error(),
		})
		return
	}

	ext := filepath.Ext(file.Filename)
	newFileName := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), os.Getpid(), ext)
	savePath := filepath.Join(saveDir, newFileName)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存文件失败: " + err.Error(),
		})
		return
	}

	fileURL := fmt.Sprintf("http://127.0.0.1:8088/photos/%s", newFileName)
	c.JSON(http.StatusOK, gin.H{
		"remote_url": fileURL,
	})
}

func pickString(v string, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
