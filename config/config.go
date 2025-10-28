package config

import (
	"log"
	"os"
)

// Config 结构体存储所有配置
type Config struct {
	OcrServiceURL string // PaddleOCR 服务的地址
	VolcanoApiKey string // 火山 API Key
	VolcanoApiURL string // 火山 API Endpoint
	QwenApiKey    string // 通义千问 API Key
	QwenApiURL    string // 通义千问 API Endpoint
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	cfg := &Config{
		// OcrServiceURL: getEnv("OCR_SERVICE_URL", "http://paddleocr-service:8888/ocr"),
		VolcanoApiKey: getEnv("VOLCANO_API_KEY", ""),
		VolcanoApiURL: getEnv("VOLCANO_API_URL", ""),
		QwenApiKey:    getEnv("QWEN_API_KEY", "sk-fc76b62ec90646d3ae38d02bfb1c3294"),
		QwenApiURL:    getEnv("QWEN_API_URL", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"),
	}

	// 本地调试时，如果 docker-compose 不在运行，可以回退到 localhost
	// 检查是否在 Docker 容器内
	// if _, exists := os.LookupEnv("IS_IN_DOCKER"); !exists {
	// 	log.Println("未在 Docker 容器中运行，OCR URL 回退到 localhost")
	// 	cfg.OcrServiceURL = getEnv("OCR_SERVICE_URL_LOCAL", "http://localhost:8888/ocr")
	// }

	return cfg
}

// 辅助函数：从环境变量读取值，如果不存在则使用默认值
func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	log.Printf("环境变量 %s 未设置, 将使用默认值: %s", key, fallback)
	return fallback
}
