package main

import (
	// module name is travel
	// use other package within the module
	"fmt"
	"log"
	"travel/internal/api"
	"travel/internal/config"
	"travel/internal/db"
	"travel/internal/ws"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"time"
)

func main() {
	// 读取配置，按需读取
	config.InitConfig()
	port := fmt.Sprintf(":%d", config.GlobalConfig.Port)
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		config.GlobalConfig.DbHost,
		config.GlobalConfig.DbUser,
		config.GlobalConfig.DbPassword,
		config.GlobalConfig.DbName,
		config.GlobalConfig.DbPort,
		config.GlobalConfig.DbSSLMode,
	)
	if err := db.InitDB(dsn); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	r := gin.Default()

	// 为了解决本地开发的CORS问题
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-User-Id", "X-Space-Id"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 注册路由与接口
	// 静态文件托管
	static_file_host_dir := config.GlobalConfig.SFHD
	r.Static("/photos", static_file_host_dir)
	// 接口注册
	r.GET("/hello", api.HttpHello)
	r.POST("/api/v1/spaces", api.HttpPostSpaces)
	r.GET("/api/v1/sync", api.HttpGetSync)
	r.POST("/api/v1/sync", api.HttpPostSync)
	r.POST("/api/v1/photos", api.HttpPostPhotos)
	r.POST("/api/v1/avatars", api.HttpPostAvatars)
	r.GET("/api/v1/ws", ws.WsSpace)

	r.Run(port)
}
