package main

import (
	"log"
	"my-ai-app/api"
	"my-ai-app/config"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// 优先从 .env 文件加载环境变量（仅用于本地开发）
	// 在生产环境中，应直接设置环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，将使用系统环境变量")
	}

	// 加载配置
	cfg := config.LoadConfig()
	if cfg.VolcanoApiKey == "" || cfg.VolcanoApiURL == "" {
		log.Fatal("火山引擎的 API Key 或 URL 未配置，请检查环境变量")
	}

	// 初始化 Gin 引擎
	router := gin.Default()

	// 初始化处理器 (注入配置)
	uploadHandler := api.NewUploadHandler(cfg)

	// 设置路由
	v1 := router.Group("/api/v1")
	{
		v1.POST("/analyze", uploadHandler.AnalyzeApplication)
	}

	// 启动服务
	log.Println("服务启动于 :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
