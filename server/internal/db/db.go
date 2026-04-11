package db

import (
	"log"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// init the database connection
// param:
// - dsn: data source name
func InitDB(dsn string) error {
	var err error

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	
	sqlDB.SetMaxIdleConns(10)  // 空闲连接池最大数量
	sqlDB.SetMaxOpenConns(100)  // 打开数据库连接的最大数量

	// 兼容历史库中的 NULL 脏数据，避免 AutoMigrate 在设置 NOT NULL 时失败。
	if err := normalizeLegacyNulls(DB); err != nil {
		log.Printf("数据库历史数据清洗失败: %v", err)
		return err
	}
	// 兼容历史库里的秒级时间戳，统一提升到毫秒，避免 sync/LWW 比较异常。
	if err := normalizeLegacyTimestampUnits(DB); err != nil {
		log.Printf("数据库时间戳单位修复失败: %v", err)
		return err
	}

	// 历史列名 server_created -> server_created_at（在 AutoMigrate 追加新列前先改名，避免双列并存）
	if err := renameLegacyServerCreatedColumn(DB); err != nil {
		log.Printf("数据库 server_created 列重命名失败: %v", err)
		return err
	}

	if err := AutoMigrateAll(); err != nil {
		log.Printf("数据库迁移失败: %v", err)
		return err
	}
	if err := copyLegacyServerCreatedIntoServerCreatedAt(DB); err != nil {
		log.Printf("数据库 server_created 数据回填到 server_created_at 失败: %v", err)
		return err
	}
	if err := backfillCommentSpaceID(DB); err != nil {
		log.Printf("comments.space_id 历史回填失败: %v", err)
		return err
	}
	if err := backfillServerSyncColumns(DB); err != nil {
		log.Printf("数据库服务端同步字段回填失败: %v", err)
		return err
	}

	log.Printf("数据库连接成功！")
	return nil
}

func backfillServerSyncColumns(db *gorm.DB) error {
	relationTables := []string{"users", "spaces", "space_members"}
	contentTables := []string{"photos", "expenses", "posts", "comments"}
	for _, table := range relationTables {
		if err := backfillServerSyncColumnsRelation(db, table); err != nil {
			return err
		}
	}
	for _, table := range contentTables {
		if err := backfillServerSyncColumnsContent(db, table); err != nil {
			return err
		}
	}
	return nil
}

func backfillServerSyncColumnsContent(db *gorm.DB, table string) error {
	hasLastModified, err := hasColumn(db, table, "last_modified")
	if err != nil {
		return err
	}
	hasServerCreatedAt, err := hasColumn(db, table, "server_created_at")
	if err != nil {
		return err
	}
	if !hasLastModified || !hasServerCreatedAt {
		return nil
	}
	queryCreated := "UPDATE " + table + " SET server_created_at = CASE WHEN created_at > 0 THEN created_at ELSE updated_at END WHERE server_created_at = 0"
	if err := db.Exec(queryCreated).Error; err != nil {
		return err
	}
	queryModified := "UPDATE " + table + " SET last_modified = GREATEST(server_created_at, updated_at, deleted_at) WHERE last_modified = 0"
	return db.Exec(queryModified).Error
}

// backfillServerSyncColumnsRelation 针对已移除客户端 created_at/updated_at 的关系表；若历史库仍保留这两列则沿用旧回填逻辑。
func backfillServerSyncColumnsRelation(db *gorm.DB, table string) error {
	hasLastModified, err := hasColumn(db, table, "last_modified")
	if err != nil {
		return err
	}
	hasServerCreatedAt, err := hasColumn(db, table, "server_created_at")
	if err != nil {
		return err
	}
	if !hasLastModified || !hasServerCreatedAt {
		return nil
	}
	hasCreatedAt, err := hasColumn(db, table, "created_at")
	if err != nil {
		return err
	}
	hasUpdatedAt, err := hasColumn(db, table, "updated_at")
	if err != nil {
		return err
	}
	if hasCreatedAt && hasUpdatedAt {
		queryCreated := "UPDATE " + table + " SET server_created_at = CASE WHEN created_at > 0 THEN created_at ELSE updated_at END WHERE server_created_at = 0"
		if err := db.Exec(queryCreated).Error; err != nil {
			return err
		}
		queryModified := "UPDATE " + table + " SET last_modified = GREATEST(server_created_at, updated_at, deleted_at) WHERE last_modified = 0"
		return db.Exec(queryModified).Error
	}
	q1 := "UPDATE " + table + " SET server_created_at = last_modified WHERE server_created_at = 0 AND last_modified > 0"
	if err := db.Exec(q1).Error; err != nil {
		return err
	}
	q2 := "UPDATE " + table + " SET last_modified = GREATEST(COALESCE(NULLIF(server_created_at,0), 0), COALESCE(NULLIF(deleted_at,0), 0)) WHERE last_modified = 0 AND (server_created_at > 0 OR deleted_at > 0)"
	return db.Exec(q2).Error
}

// renameLegacyServerCreatedColumn 将旧列 server_created 改名为 server_created_at（仅当新列尚不存在时）。
func renameLegacyServerCreatedColumn(db *gorm.DB) error {
	tables := []string{"users", "spaces", "space_members", "photos", "expenses", "posts", "comments"}
	for _, table := range tables {
		hasOld, err := hasColumn(db, table, "server_created")
		if err != nil {
			return err
		}
		hasNew, err := hasColumn(db, table, "server_created_at")
		if err != nil {
			return err
		}
		if !hasOld || hasNew {
			continue
		}
		q := "ALTER TABLE " + table + " RENAME COLUMN server_created TO server_created_at"
		if err := db.Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}

// copyLegacyServerCreatedIntoServerCreatedAt 在同时存在 server_created 与 server_created_at 时，把旧列数据拷入新列（AutoMigrate 可能已追加空的新列）。
func copyLegacyServerCreatedIntoServerCreatedAt(db *gorm.DB) error {
	tables := []string{"users", "spaces", "space_members", "photos", "expenses", "posts", "comments"}
	for _, table := range tables {
		hasOld, err := hasColumn(db, table, "server_created")
		if err != nil {
			return err
		}
		hasNew, err := hasColumn(db, table, "server_created_at")
		if err != nil {
			return err
		}
		if !hasOld || !hasNew {
			continue
		}
		q := "UPDATE " + table + " SET server_created_at = server_created WHERE server_created > 0 AND server_created_at = 0"
		if err := db.Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}

// backfillCommentSpaceID 用 post 的 space_id 补齐历史 comments（新设计下 comments 需直接带 space_id）。
func backfillCommentSpaceID(db *gorm.DB) error {
	hasCol, err := hasColumn(db, "comments", "space_id")
	if err != nil || !hasCol {
		return err
	}
	q := `UPDATE comments AS c SET space_id = p.space_id FROM posts AS p WHERE c.post_id = p.id AND (c.space_id IS NULL OR c.space_id = '')`
	return db.Exec(q).Error
}

type nullPatch struct {
	table        string
	column       string
	defaultValue string
}

func normalizeLegacyNulls(db *gorm.DB) error {
	patches := []nullPatch{
		{table: "users", column: "nickname", defaultValue: "''"},
		// avatar_remote_url 已废弃；历史库若仍有该列，继续把 NULL 洗净以免约束失败
		{table: "users", column: "avatar_remote_url", defaultValue: "''"},
		{table: "users", column: "created_at", defaultValue: "0"},
		{table: "users", column: "updated_at", defaultValue: "0"},
		{table: "users", column: "deleted_at", defaultValue: "0"},

		{table: "spaces", column: "name", defaultValue: "''"},
		{table: "spaces", column: "created_at", defaultValue: "0"},
		{table: "spaces", column: "updated_at", defaultValue: "0"},
		{table: "spaces", column: "deleted_at", defaultValue: "0"},

		{table: "space_members", column: "space_id", defaultValue: "''"},
		{table: "space_members", column: "user_id", defaultValue: "''"},
		{table: "space_members", column: "created_at", defaultValue: "0"},
		{table: "space_members", column: "updated_at", defaultValue: "0"},
		{table: "space_members", column: "deleted_at", defaultValue: "0"},

		{table: "posts", column: "space_id", defaultValue: "''"},
		// poster_id / content 已废弃；历史列若仍存在则洗净 NULL
		{table: "posts", column: "poster_id", defaultValue: "''"},
		{table: "posts", column: "content", defaultValue: "''"},
		{table: "posts", column: "created_at", defaultValue: "0"},
		{table: "posts", column: "updated_at", defaultValue: "0"},
		{table: "posts", column: "deleted_at", defaultValue: "0"},

		{table: "photos", column: "space_id", defaultValue: "''"},
		{table: "photos", column: "uploader_id", defaultValue: "''"},
		{table: "photos", column: "remote_url", defaultValue: "''"},
		{table: "photos", column: "post_id", defaultValue: "''"},
		{table: "photos", column: "shoted_at", defaultValue: "0"},
		{table: "photos", column: "created_at", defaultValue: "0"},
		{table: "photos", column: "updated_at", defaultValue: "0"},
		{table: "photos", column: "deleted_at", defaultValue: "0"},

		{table: "expenses", column: "space_id", defaultValue: "''"},
		{table: "expenses", column: "payer_id", defaultValue: "''"},
		{table: "expenses", column: "description", defaultValue: "''"},
		{table: "expenses", column: "amount", defaultValue: "0"},
		{table: "expenses", column: "created_at", defaultValue: "0"},
		{table: "expenses", column: "updated_at", defaultValue: "0"},
		{table: "expenses", column: "deleted_at", defaultValue: "0"},

		{table: "comments", column: "space_id", defaultValue: "''"},
		{table: "comments", column: "content", defaultValue: "''"},
		{table: "comments", column: "commenter_id", defaultValue: "''"},
		{table: "comments", column: "post_id", defaultValue: "''"},
		{table: "comments", column: "commented_at", defaultValue: "0"},
		{table: "comments", column: "created_at", defaultValue: "0"},
		{table: "comments", column: "updated_at", defaultValue: "0"},
		{table: "comments", column: "deleted_at", defaultValue: "0"},
	}

	for _, p := range patches {
		exists, err := hasColumn(db, p.table, p.column)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		query := "UPDATE " + p.table + " SET " + p.column + " = " + p.defaultValue + " WHERE " + p.column + " IS NULL"
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}
	return nil
}

func hasColumn(db *gorm.DB, table string, column string) (bool, error) {
	var count int64
	err := db.Raw(
		`SELECT COUNT(1) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		strings.ToLower(table),
		strings.ToLower(column),
	).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func normalizeLegacyTimestampUnits(db *gorm.DB) error {
	// 小于 1e12 的正时间戳视作秒级，统一转成毫秒。
	cols := []struct {
		table  string
		column string
	}{
		{"users", "created_at"}, {"users", "updated_at"}, {"users", "deleted_at"},
		{"users", "last_modified"}, {"users", "server_created"}, {"users", "server_created_at"},
		{"spaces", "created_at"}, {"spaces", "updated_at"}, {"spaces", "deleted_at"},
		{"spaces", "last_modified"}, {"spaces", "server_created"}, {"spaces", "server_created_at"},
		{"space_members", "created_at"}, {"space_members", "updated_at"}, {"space_members", "deleted_at"},
		{"space_members", "last_modified"}, {"space_members", "server_created"}, {"space_members", "server_created_at"},
		{"posts", "created_at"}, {"posts", "updated_at"}, {"posts", "deleted_at"},
		{"posts", "last_modified"}, {"posts", "server_created"}, {"posts", "server_created_at"},
		{"photos", "shoted_at"}, {"photos", "created_at"}, {"photos", "updated_at"}, {"photos", "deleted_at"},
		{"photos", "last_modified"}, {"photos", "server_created"}, {"photos", "server_created_at"},
		{"comments", "commented_at"}, {"comments", "created_at"}, {"comments", "updated_at"}, {"comments", "deleted_at"},
		{"comments", "last_modified"}, {"comments", "server_created"}, {"comments", "server_created_at"},
		{"expenses", "created_at"}, {"expenses", "updated_at"}, {"expenses", "deleted_at"},
		{"expenses", "last_modified"}, {"expenses", "server_created"}, {"expenses", "server_created_at"},
	}
	for _, c := range cols {
		exists, err := hasColumn(db, c.table, c.column)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		query := "UPDATE " + c.table + " SET " + c.column + " = " + c.column + " * 1000 WHERE " + c.column + " > 0 AND " + c.column + " < 1000000000000"
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}
	return nil
}