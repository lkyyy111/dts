package sync

import (
	"encoding/json"
	"travel/internal/db"
)

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
	LastPulledAt int64                       `json:"last_pulled_at"`
	Changes      map[string]SyncChangeBucket `json:"changes"`
}

type SyncChangeBucket struct {
	Created []json.RawMessage `json:"created"`
	Updated []json.RawMessage `json:"updated"`
	Deleted []string          `json:"deleted"`
}

// SyncUser 核心关系表：仅与 Pull 返回字段一致。
type SyncUser struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

type SyncSpace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SyncSpaceMember struct {
	ID      string `json:"id"`
	SpaceID string `json:"space_id"`
	UserID  string `json:"user_id"`
}

type SyncPhoto struct {
	ID         string `json:"id"`
	SpaceID    string `json:"space_id"`
	UploaderID string `json:"uploader_id"`
	RemoteURL  string `json:"remote_url"`
	PostID     string `json:"post_id"`
	ShotedAt   int64  `json:"shoted_at"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

type SyncExpense struct {
	ID          string  `json:"id"`
	SpaceID     string  `json:"space_id"`
	PayerID     string  `json:"payer_id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

type SyncPost struct {
	ID        string `json:"id"`
	SpaceID   string `json:"space_id"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type SyncComment struct {
	ID          string `json:"id"`
	SpaceID     string `json:"space_id"`
	Content     string `json:"content"`
	CommenterID string `json:"commenter_id"`
	PostID      string `json:"post_id"`
	CommentedAt int64  `json:"commented_at"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
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

type PushMode int

const (
	PushModeCreated PushMode = iota
	PushModeUpdated
)
