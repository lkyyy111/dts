package db

import "gorm.io/gorm"

// User 为核心关系表：仅 id / nickname，不参与增量同步字段与软删。
type User struct {
	ID       string `json:"id" gorm:"type:varchar(26);primaryKey"`
	Nickname string `json:"nickname" gorm:"type:text;not null;default:''"`
}

func (User) TableName() string { return "users" }

// Space 为核心关系表：仅 id / name。
type Space struct {
	ID   string `json:"id" gorm:"type:varchar(26);primaryKey"`
	Name string `json:"name" gorm:"type:text;not null;default:''"`
}

func (Space) TableName() string { return "spaces" }

// SpaceMember 为核心关系表：id 规范为 {space_id}_{user_id}。
type SpaceMember struct {
	ID      string `json:"id" gorm:"type:text;primaryKey"`
	SpaceID string `json:"space_id" gorm:"column:space_id;type:varchar(26);not null;index"`
	UserID  string `json:"user_id" gorm:"column:user_id;type:varchar(26);not null;index"`
}

func (SpaceMember) TableName() string { return "space_members" }

type Post struct {
	ID              string `json:"id" gorm:"type:varchar(26);primaryKey"`
	SpaceID         string `json:"space_id" gorm:"column:space_id;type:varchar(26);not null;index:idx_posts_space_updated,priority:1;index:idx_posts_space_deleted,priority:1"`
	CreatedAt       int64  `json:"created_at" gorm:"column:created_at;not null;index;autoCreateTime:false"`
	UpdatedAt       int64  `json:"updated_at" gorm:"column:updated_at;not null;index:idx_posts_space_updated,priority:2;autoUpdateTime:false"`
	DeletedAt       int64  `json:"-" gorm:"column:deleted_at;not null;default:0;index:idx_posts_space_deleted,priority:2"`
	LastModified    int64  `json:"-" gorm:"column:last_modified;not null;default:0;index"`
	ServerCreatedAt int64  `json:"-" gorm:"column:server_created_at;not null;default:0;index"`
}

func (Post) TableName() string { return "posts" }

type Photo struct {
	ID              string `json:"id" gorm:"type:varchar(26);primaryKey"`
	SpaceID         string `json:"space_id" gorm:"column:space_id;type:varchar(26);not null;index:idx_photos_space_updated,priority:1;index:idx_photos_space_deleted,priority:1"`
	UploaderID      string `json:"uploader_id" gorm:"column:uploader_id;type:varchar(26);not null;index"`
	RemoteURL       string `json:"remote_url" gorm:"column:remote_url;type:text;not null;default:''"`
	PostID          string `json:"post_id" gorm:"column:post_id;type:varchar(26);not null;default:'';index"`
	ShotedAt        int64  `json:"shoted_at" gorm:"column:shoted_at;not null;default:0;index"`
	CreatedAt       int64  `json:"created_at" gorm:"column:created_at;not null;index;autoCreateTime:false"`
	UpdatedAt       int64  `json:"updated_at" gorm:"column:updated_at;not null;index:idx_photos_space_updated,priority:2;autoUpdateTime:false"`
	DeletedAt       int64  `json:"-" gorm:"column:deleted_at;not null;default:0;index:idx_photos_space_deleted,priority:2"`
	LastModified    int64  `json:"-" gorm:"column:last_modified;not null;default:0;index"`
	ServerCreatedAt int64  `json:"-" gorm:"column:server_created_at;not null;default:0;index"`
}

func (Photo) TableName() string { return "photos" }

type Expense struct {
	ID              string  `json:"id" gorm:"type:varchar(26);primaryKey"`
	SpaceID         string  `json:"space_id" gorm:"column:space_id;type:varchar(26);not null;index:idx_expenses_space_updated,priority:1;index:idx_expenses_space_deleted,priority:1"`
	PayerID         string  `json:"payer_id" gorm:"column:payer_id;type:varchar(26);not null;index"`
	Amount          float64 `json:"amount" gorm:"column:amount;type:numeric(12,2);not null"`
	Description     string  `json:"description" gorm:"column:description;type:text;not null;default:''"`
	CreatedAt       int64   `json:"created_at" gorm:"column:created_at;not null;index;autoCreateTime:false"`
	UpdatedAt       int64   `json:"updated_at" gorm:"column:updated_at;not null;index:idx_expenses_space_updated,priority:2;autoUpdateTime:false"`
	DeletedAt       int64   `json:"-" gorm:"column:deleted_at;not null;default:0;index:idx_expenses_space_deleted,priority:2"`
	LastModified    int64   `json:"-" gorm:"column:last_modified;not null;default:0;index"`
	ServerCreatedAt int64   `json:"-" gorm:"column:server_created_at;not null;default:0;index"`
}

func (Expense) TableName() string { return "expenses" }

type Comment struct {
	ID              string `json:"id" gorm:"type:varchar(26);primaryKey"`
	SpaceID         string `json:"space_id" gorm:"column:space_id;type:varchar(26);not null;index:idx_comments_space_updated,priority:1;index:idx_comments_space_deleted,priority:1"`
	Content         string `json:"content" gorm:"column:content;type:text;not null;default:''"`
	CommenterID     string `json:"commenter_id" gorm:"column:commenter_id;type:varchar(26);not null;index"`
	PostID          string `json:"post_id" gorm:"column:post_id;type:varchar(26);not null;index"`
	CommentedAt     int64  `json:"commented_at" gorm:"column:commented_at;not null;default:0;index"`
	CreatedAt       int64  `json:"created_at" gorm:"column:created_at;not null;index;autoCreateTime:false"`
	UpdatedAt       int64  `json:"updated_at" gorm:"column:updated_at;not null;index:idx_comments_space_updated,priority:2;autoUpdateTime:false"`
	DeletedAt       int64  `json:"-" gorm:"column:deleted_at;not null;default:0;index:idx_comments_space_deleted,priority:2"`
	LastModified    int64  `json:"-" gorm:"column:last_modified;not null;default:0;index"`
	ServerCreatedAt int64  `json:"-" gorm:"column:server_created_at;not null;default:0;index"`
}

func (Comment) TableName() string { return "comments" }

func AutoMigrateAll() error {
	return DB.AutoMigrate(
		&User{},
		&Space{},
		&SpaceMember{},
		&Post{},
		&Photo{},
		&Expense{},
		&Comment{},
	)
}

func WithTx(fn func(tx *gorm.DB) error) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}
