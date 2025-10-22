package service

import (
	"fmt"
	"log"
	"mime/multipart"
	"my-ai-app/client"
	"my-ai-app/config"
	"my-ai-app/model"
	"my-ai-app/rules"
)

type AnalysisService struct {
	volcanoClient *client.VolcanoClient
	oaClient      *client.OaClient
}

func NewAnalysisService(cfg *config.Config) *AnalysisService {
	return &AnalysisService{
		volcanoClient: client.NewVolcanoClient(cfg.VolcanoApiURL, cfg.VolcanoApiKey),
		oaClient:      client.NewOaClient(cfg.OaApiBaseUrl),
	}
}

// Analyze 核心业务逻辑 (签名已改变)
func (s *AnalysisService) Analyze(appData model.ApplicationData, fileHeader *multipart.FileHeader) (*model.AnalysisResult, error) {

	var extractedImageData *model.ExtractedData
	var err error

	// 传入的 alias 已经是 OA 系统查询到的员工姓名，无需再次验证
	log.Printf("申请员工姓名: %s", appData.Alias)

	// 1. 检查是否传入了图片
	if fileHeader != nil {
		// 如果有图片，调用火山 AI "提取数据"
		// 总是让 AI 识别图片中的所有姓名，然后与传入的 alias 进行比对
		extractedImageData, err = s.volcanoClient.ExtractDataFromImage(fileHeader, "")
		if err != nil {
			return nil, fmt.Errorf("火山 AI 提取图片数据失败: %w", err)
		}
	}

	// 2. 调用规则引擎 (Go 代码)
	// 将 "表单数据" 和 "AI提取的图片数据" (可能为 nil) 一起传入
	result := rules.ValidateApplication(appData, extractedImageData)

	return result, nil
}
