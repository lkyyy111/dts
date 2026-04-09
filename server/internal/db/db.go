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

	if err := AutoMigrateAll(); err != nil {
		log.Printf("数据库迁移失败: %v", err)
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
	tables := []string{"users", "spaces", "space_members", "photos", "expenses", "posts", "comments"}
	for _, table := range tables {
		hasLastModified, err := hasColumn(db, table, "last_modified")
		if err != nil {
			return err
		}
		hasServerCreated, err := hasColumn(db, table, "server_created")
		if err != nil {
			return err
		}
		if !hasLastModified || !hasServerCreated {
			continue
		}
		queryCreated := "UPDATE " + table + " SET server_created = CASE WHEN created_at > 0 THEN created_at ELSE updated_at END WHERE server_created = 0"
		if err := db.Exec(queryCreated).Error; err != nil {
			return err
		}
		queryModified := "UPDATE " + table + " SET last_modified = GREATEST(server_created, updated_at, deleted_at) WHERE last_modified = 0"
		if err := db.Exec(queryModified).Error; err != nil {
			return err
		}
	}
	return nil
}

type nullPatch struct {
	table        string
	column       string
	defaultValue string
}

func normalizeLegacyNulls(db *gorm.DB) error {
	patches := []nullPatch{
		{table: "users", column: "nickname", defaultValue: "''"},
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
		{table: "posts", column: "poster_id", defaultValue: "''"},
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
		{"spaces", "created_at"}, {"spaces", "updated_at"}, {"spaces", "deleted_at"},
		{"space_members", "created_at"}, {"space_members", "updated_at"}, {"space_members", "deleted_at"},
		{"posts", "created_at"}, {"posts", "updated_at"}, {"posts", "deleted_at"},
		{"photos", "shoted_at"}, {"photos", "created_at"}, {"photos", "updated_at"}, {"photos", "deleted_at"},
		{"comments", "commented_at"}, {"comments", "created_at"}, {"comments", "updated_at"}, {"comments", "deleted_at"},
		{"expenses", "created_at"}, {"expenses", "updated_at"}, {"expenses", "deleted_at"},
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