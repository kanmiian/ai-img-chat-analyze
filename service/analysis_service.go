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
	timeValidator *TimeValidator        // 时间验证器
}

// NewAnalysisService 注入所有客户端
func NewAnalysisService(cfg *config.Config) *AnalysisService {
	oaClient := client.NewOaClient(cfg.OaApiBaseUrl)
	return &AnalysisService{
		qwenClient:    client.NewQwenClient(cfg.QwenApiURL, cfg.QwenApiKey),
		volcanoClient: client.NewVolcanoClient(cfg.VolcanoApiURL, cfg.VolcanoApiKey),
		oaClient:      oaClient,
		timeValidator: NewTimeValidator(oaClient),
	}
}

// --- 调用 Qwen ---
func (s *AnalysisService) AnalyzeWithQwen(appData model.ApplicationData, fileHeaders []*multipart.FileHeader) (*model.AnalysisResult, error) {
	// 调用私有助手，并指定 "qwen"
	return s.runAnalysis(appData, fileHeaders, "qwen")
}

// --- 调用 Volcano ---
func (s *AnalysisService) AnalyzeWithVolcano(appData model.ApplicationData, fileHeaders []*multipart.FileHeader) (*model.AnalysisResult, error) {
	// 调用私有助手，并指定 "volcano"
	return s.runAnalysis(appData, fileHeaders, "volcano")
}

func (s *AnalysisService) runAnalysis(appData model.ApplicationData, fileHeaders []*multipart.FileHeader, provider string) (*model.AnalysisResult, error) {
	startTime := time.Now()
	log.Printf("开始分析请求 - Provider: %s, UserId: %s, Alias: %s, Type: %s, 图片数量: %d",
		provider, appData.UserId, appData.Alias, appData.ApplicationType, len(fileHeaders))

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

	// 2. 检查是否有图片输入
	hasImages := len(fileHeaders) > 0 || len(appData.ImageUrls) > 0

	// 3. 如果没有图片，检查是否必需
	if !hasImages {
		if appData.ApplicationType == "病假" || appData.ApplicationType == "补打卡" {
			log.Printf("缺少必要图片 - Type: %s", appData.ApplicationType)
			return &model.AnalysisResult{IsAbnormal: true, Reason: "病假和补打卡申请必须提供证明材料图片"}, nil
		}
		// 没有图片且不需要图片，返回正常结果
		log.Printf("无需图片验证 - Type: %s", appData.ApplicationType)
		return &model.AnalysisResult{IsAbnormal: false, Reason: "正常"}, nil
	}

	// 4. 准备 AI 分析所需的参数
	employeeName := ""
	if appData.Alias != "" {
		employeeName = appData.Alias
	} else if oaEmployeeData != nil {
		employeeName = oaEmployeeData.Alias
	}

	// 5. 处理多张图片，只要有一张满足条件即可
	var validExtractedData *model.ExtractedData
	var validImageIndex int
	var imagesAnalysis []model.ImageAnalysisDetail
	totalImages := len(fileHeaders) + len(appData.ImageUrls)

	log.Printf("开始AI分析 - Provider: %s, EmployeeName: %s, 总图片数: %d (文件: %d, URL: %d)",
		provider, employeeName, totalImages, len(fileHeaders), len(appData.ImageUrls))

	// 5.1 处理上传的文件
	for i, fileHeader := range fileHeaders {
		imageIndex := i + 1
		aiStartTime := time.Now()
		log.Printf("分析第 %d/%d 张图片（文件上传，文件名: %s, 大小: %d bytes）",
			imageIndex, totalImages, fileHeader.Filename, fileHeader.Size)

		detail := model.ImageAnalysisDetail{
			Index:    imageIndex,
			Source:   "file_upload",
			FileName: fileHeader.Filename,
		}

		var extractedData *model.ExtractedData
		var err error

		switch provider {
		case "qwen":
			extractedData, err = s.qwenClient.ExtractDataFromImage(fileHeader, "", employeeName, appData.ApplicationType)
		case "volcano":
			extractedData, err = s.volcanoClient.ExtractDataFromImage(fileHeader, "", employeeName, appData.ApplicationType)
		default:
			return nil, fmt.Errorf("未知的 AI provider: %s", provider)
		}

		aiDuration := time.Since(aiStartTime)
		detail.ProcessingTimeMs = aiDuration.Milliseconds()

		if err != nil {
			detail.Success = false
			detail.ErrorMessage = err.Error()
			detail.IsValid = false
			imagesAnalysis = append(imagesAnalysis, detail)

			log.Printf("✗ 第 %d 张图片分析失败 (耗时: %v): %v", imageIndex, aiDuration, err)
			continue
		}

		// 分析成功
		detail.Success = true
		detail.ExtractedData = extractedData
		detail.IsValid = extractedData.IsProofTypeValid
		imagesAnalysis = append(imagesAnalysis, detail)

		log.Printf("第 %d 张图片分析完成 (耗时: %v): IsProofTypeValid=%v, ExtractedName=%s, RequestType=%s, Content=%s",
			imageIndex, aiDuration, extractedData.IsProofTypeValid, extractedData.ExtractedName,
			extractedData.RequestType, extractedData.Content)

		// 检查是否满足条件（证明材料类型有效）
		if validImageIndex == 0 && extractedData.IsProofTypeValid {
			validExtractedData = extractedData
			validImageIndex = imageIndex
			log.Printf("✓ 第 %d 张图片满足条件，停止处理后续图片", imageIndex)
			break
		}
	}

	// 5.2 处理 URL 图片
	if validExtractedData == nil {
		for i, imageURL := range appData.ImageUrls {
			imageIndex := len(fileHeaders) + i + 1
			aiStartTime := time.Now()
			log.Printf("分析第 %d/%d 张图片（URL下载: %s）", imageIndex, totalImages, imageURL)

			detail := model.ImageAnalysisDetail{
				Index:    imageIndex,
				Source:   "url_download",
				ImageURL: imageURL,
			}

			var extractedData *model.ExtractedData
			var err error

			switch provider {
			case "qwen":
				extractedData, err = s.qwenClient.ExtractDataFromImage(nil, imageURL, employeeName, appData.ApplicationType)
			case "volcano":
				extractedData, err = s.volcanoClient.ExtractDataFromImage(nil, imageURL, employeeName, appData.ApplicationType)
			default:
				return nil, fmt.Errorf("未知的 AI provider: %s", provider)
			}

			aiDuration := time.Since(aiStartTime)
			detail.ProcessingTimeMs = aiDuration.Milliseconds()

			if err != nil {
				detail.Success = false
				detail.ErrorMessage = err.Error()
				detail.IsValid = false
				imagesAnalysis = append(imagesAnalysis, detail)

				log.Printf("✗ 第 %d 张图片分析失败 (耗时: %v): %v", imageIndex, aiDuration, err)
				continue
			}

			// 分析成功
			detail.Success = true
			detail.ExtractedData = extractedData
			detail.IsValid = extractedData.IsProofTypeValid
			imagesAnalysis = append(imagesAnalysis, detail)

			log.Printf("第 %d 张图片分析完成 (耗时: %v): IsProofTypeValid=%v, ExtractedName=%s, RequestType=%s, Content=%s",
				imageIndex, aiDuration, extractedData.IsProofTypeValid, extractedData.ExtractedName,
				extractedData.RequestType, extractedData.Content)

			// 检查是否满足条件（证明材料类型有效）
			if extractedData.IsProofTypeValid {
				validExtractedData = extractedData
				validImageIndex = imageIndex
				log.Printf("✓ 第 %d 张图片满足条件，停止处理后续图片", imageIndex)
				break
			}
		}
	}

	// 6. 如果所有图片都不满足条件
	if validExtractedData == nil {
		log.Printf("所有图片均不满足条件")

		// 统计错误数量
		errorCount := 0
		for _, detail := range imagesAnalysis {
			if !detail.Success {
				errorCount++
			}
		}

		if errorCount == len(imagesAnalysis) && errorCount > 0 {
			// 所有图片都分析失败
			log.Printf("所有 %d 张图片分析均失败", errorCount)
			return &model.AnalysisResult{
				IsAbnormal:     true,
				Reason:         "所有图片分析均失败",
				ImagesAnalysis: imagesAnalysis,
			}, nil
		}

		// 图片都处理成功了，但都不满足条件
		log.Printf("所有图片分析成功，但均不是有效的证明材料")
		return &model.AnalysisResult{
			IsAbnormal:     true,
			Reason:         "所有提供的图片均不是有效的证明材料",
			ImagesAnalysis: imagesAnalysis,
		}, nil
	}

	// 7. 调用规则引擎进行最终裁决（只依赖 AI 判断，不使用 time_validation）
	rulesStartTime := time.Now()
	log.Printf("开始规则引擎验证")
	result := rules.ValidateApplication(appData, validExtractedData)
	rulesDuration := time.Since(rulesStartTime)

	// 8. 添加详细分析结果
	result.ValidImageIndex = validImageIndex
	result.ImagesAnalysis = imagesAnalysis

	totalDuration := time.Since(startTime)
	log.Printf("规则引擎验证完成 (耗时: %v)", rulesDuration)
	log.Printf("总分析时间: %v, 结果: IsAbnormal=%v, Reason=%s, 有效图片索引=%d",
		totalDuration, result.IsAbnormal, result.Reason, validImageIndex)

	return result, nil
}
