package db

import (
	"testing"
)

// This function tests the connection of the database
// It does NOT read the config
func TestInitDB(t *testing.T) {
	dsn := "host=localhost user=dts password=dts123 dbname=dts port=5432 sslmode=disable TimeZone=Asia/Shanghai"

	err := InitDB(dsn)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	if DB == nil {
		t.Fatal("DB实例为空，连接逻辑有误")
	}

	// 获取底层数据库
	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("获取底层 sql.DB 失败: %v", err)
	}

	// ping数据库
	err = sqlDB.Ping()
	if err != nil {
		t.Fatalf("无法 Ping 通数据库，请检查数据库是否运行: %v", err)
	}
}