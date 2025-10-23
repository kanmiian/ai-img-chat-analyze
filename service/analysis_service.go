package service

import (
	"fmt"
	"log"
	"mime/multipart"
	"my-ai-app/client"
	"my-ai-app/config"
	"my-ai-app/model"
	"my-ai-app/rules"
	"time"
)

// AnalysisService 同时持有所有客户端
type AnalysisService struct {
	qwenClient    *client.QwenClient    // 通义千问客户端
	volcanoClient *client.VolcanoClient // 火山引擎客户端
	oaClient      *client.OaClient      // OA 客户端
}

// NewAnalysisService 注入所有客户端
func NewAnalysisService(cfg *config.Config) *AnalysisService {
	return &AnalysisService{
		qwenClient:    client.NewQwenClient(cfg.QwenApiURL, cfg.QwenApiKey),
		volcanoClient: client.NewVolcanoClient(cfg.VolcanoApiURL, cfg.VolcanoApiKey),
		oaClient:      client.NewOaClient(cfg.OaApiBaseUrl),
	}
}

// --- 调用 Qwen ---
func (s *AnalysisService) AnalyzeWithQwen(appData model.ApplicationData, fileHeader *multipart.FileHeader) (*model.AnalysisResult, error) {
	// 调用私有助手，并指定 "qwen"
	return s.runAnalysis(appData, fileHeader, "qwen")
}

// --- 调用 Volcano ---
func (s *AnalysisService) AnalyzeWithVolcano(appData model.ApplicationData, fileHeader *multipart.FileHeader) (*model.AnalysisResult, error) {
	// 调用私有助手，并指定 "volcano"
	return s.runAnalysis(appData, fileHeader, "volcano")
}

func (s *AnalysisService) runAnalysis(appData model.ApplicationData, fileHeader *multipart.FileHeader, provider string) (*model.AnalysisResult, error) {
	startTime := time.Now()
	log.Printf("开始分析请求 - Provider: %s, UserId: %s, Alias: %s, Type: %s",
		provider, appData.UserId, appData.Alias, appData.ApplicationType)

	// 1. 尝试从 OA 系统获取员工考勤基准数据（可选，失败不影响主流程）
	oaStartTime := time.Now()
	var oaEmployeeData *client.EmployeeData
	if appData.UserId != "" {
		oaData, err := s.oaClient.GetEmployeeData(appData.UserId)
		oaDuration := time.Since(oaStartTime)
		if err != nil {
			log.Printf("OA 系统获取员工数据失败 (耗时: %v): %v", oaDuration, err)
		} else {
			oaEmployeeData = oaData
			log.Printf("OA 系统获取到员工数据 (耗时: %v): %+v", oaDuration, oaEmployeeData)
		}
	} else {
		log.Printf("跳过OA查询 - 未提供UserId")
	}

	var extractedImageData *model.ExtractedData // 默认为 nil

	// 2. 如果有图片，调用 AI 客户端进行分析（无论是否有姓名都要分析）
	if fileHeader != nil {
		// 准备AI分析所需的参数
		employeeName := ""
		if appData.Alias != "" {
			employeeName = appData.Alias
		} else if oaEmployeeData != nil {
			employeeName = oaEmployeeData.Alias
		}

		aiStartTime := time.Now()
		log.Printf("开始AI分析 - Provider: %s, EmployeeName: %s, FileSize: %d bytes",
			provider, employeeName, fileHeader.Size)

		switch provider {
		case "qwen":
			var err error
			extractedImageData, err = s.qwenClient.ExtractDataFromImage(fileHeader, employeeName, appData.ApplicationType)
			aiDuration := time.Since(aiStartTime)
			if err != nil {
				log.Printf("Qwen AI 提取失败 (耗时: %v): %v", aiDuration, err)
				return nil, fmt.Errorf("Qwen AI 提取失败: %w", err)
			}
			log.Printf("Qwen AI 分析完成 (耗时: %v)", aiDuration)
		case "volcano":
			var err error
			extractedImageData, err = s.volcanoClient.ExtractDataFromImage(fileHeader, employeeName, appData.ApplicationType)
			aiDuration := time.Since(aiStartTime)
			if err != nil {
				log.Printf("Volcano AI 提取失败 (耗时: %v): %v", aiDuration, err)
				return nil, fmt.Errorf("Volcano AI 提取失败: %w", err)
			}
			log.Printf("Volcano AI 分析完成 (耗时: %v)", aiDuration)
		default:
			return nil, fmt.Errorf("未知的 AI provider: %s", provider)
		}

		log.Printf("AI 提取的图片数据: %+v", extractedImageData)
	} else {
		// 3. 如果没有图片，检查是否必需
		if appData.ApplicationType == "病假" || appData.ApplicationType == "补打卡" {
			log.Printf("缺少必要图片 - Type: %s", appData.ApplicationType)
			return &model.AnalysisResult{IsAbnormal: true, Reason: "病假和补打卡申请必须提供证明材料图片"}, nil
		}
	}

	// 4. 调用规则引擎进行最终裁决
	rulesStartTime := time.Now()
	log.Printf("开始规则引擎验证")
	result := rules.ValidateApplication(appData, extractedImageData)
	rulesDuration := time.Since(rulesStartTime)

	totalDuration := time.Since(startTime)
	log.Printf("规则引擎验证完成 (耗时: %v)", rulesDuration)
	log.Printf("总分析时间: %v, 结果: IsAbnormal=%v, Reason=%s",
		totalDuration, result.IsAbnormal, result.Reason)

	return result, nil
}
