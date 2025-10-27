package main

import (
	"log"
	"my-ai-app/api"
	"my-ai-app/config"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，将使用系统环境变量")
	}

	cfg := config.LoadConfig()
	if cfg.QwenApiKey == "" || cfg.VolcanoApiKey == "" {
		log.Println("警告: Qwen 或 Volcano 的 API Key 未配置，相关接口可能无法工作")
	}

	router := gin.Default()
	uploadHandler := api.NewUploadHandler(cfg)

	v1 := router.Group("/api/v1")
	{
		v1.POST("/analyze-qwen", uploadHandler.AnalyzeQwen)
		v1.POST("/analyze-volcano", uploadHandler.AnalyzeVolcano)
		v1.POST("/test-volcano", uploadHandler.TestVolcanoSimple) // 火山引擎测试接口
	}

	log.Println("服务启动于 :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
