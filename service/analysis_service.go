package service

import (
	"fmt"
	"log"
	"mime/multipart"
	"my-ai-app/client"
	"my-ai-app/config"
	"my-ai-app/model"
	"my-ai-app/rules"
	"sync"
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

	// 1. 检查是否有图片输入
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

	// 2. 准备 AI 分析所需的参数
	employeeName := appData.Alias

	// 5. 并发处理多张图片
	var validImageIndex int
	var imagesAnalysis []model.ImageAnalysisDetail
	totalImages := len(fileHeaders) + len(appData.ImageUrls)

	log.Printf("开始AI并发分析 - Provider: %s, EmployeeName: %s, 总图片数: %d (文件: %d, URL: %d)",
		provider, employeeName, totalImages, len(fileHeaders), len(appData.ImageUrls))

	// 使用channel和goroutine并发处理
	type analysisResult struct {
		detail        model.ImageAnalysisDetail
		extractedData *model.ExtractedData
		index         int
		err           error
	}

	resultChan := make(chan analysisResult, totalImages)
	var wg sync.WaitGroup

	// 5.1 并发处理上传的文件
	for i, fileHeader := range fileHeaders {
		wg.Add(1)
		go func(index int, fh *multipart.FileHeader) {
			defer wg.Done()

			aiStartTime := time.Now()
			log.Printf("并发分析第 %d/%d 张图片（文件上传，文件名: %s, 大小: %d bytes）",
				index+1, totalImages, fh.Filename, fh.Size)

			detail := model.ImageAnalysisDetail{
				Index:    index + 1,
				Source:   "file_upload",
				FileName: fh.Filename,
			}

			var extractedData *model.ExtractedData
			var requestId string
			var tokenUsage *model.TokenUsage
			var err error

			switch provider {
			case "qwen":
				extractedData, requestId, tokenUsage, err = s.qwenClient.ExtractDataFromImage(fh, "", employeeName, appData.ApplicationType, appData.ApplicationDate, appData.StartTime, appData.EndTime)
			case "volcano":
				extractedData, requestId, tokenUsage, err = s.volcanoClient.ExtractDataFromImage(fh, "", employeeName, appData.ApplicationType, appData.ApplicationDate, appData.StartTime, appData.EndTime)
			default:
				err = fmt.Errorf("未知的 AI provider: %s", provider)
			}

			// 设置requestId和tokenUsage
			detail.RequestId = requestId
			detail.TokenUsage = tokenUsage

			aiDuration := time.Since(aiStartTime)
			detail.ProcessingTimeMs = aiDuration.Milliseconds()

			if err != nil {
				detail.Success = false
				detail.ErrorMessage = err.Error()
				detail.IsValid = false
				log.Printf("✗ 第 %d 张图片分析失败 (耗时: %v): %v", index+1, aiDuration, err)
			} else {
				// 分析成功
				detail.Success = true
				detail.ExtractedData = extractedData
				detail.IsValid = extractedData.IsProofTypeValid
				log.Printf("第 %d 张图片分析完成 (耗时: %v): IsProofTypeValid=%v, ExtractedName=%s, RequestType=%s",
					index+1, aiDuration, extractedData.IsProofTypeValid, extractedData.ExtractedName, extractedData.RequestType)
			}

			resultChan <- analysisResult{
				detail:        detail,
				extractedData: extractedData,
				index:         index,
				err:           err,
			}
		}(i, fileHeader)
	}

	// 5.2 并发处理 URL 图片
	for i, imageURL := range appData.ImageUrls {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()

			imageIndex := len(fileHeaders) + index + 1
			aiStartTime := time.Now()
			log.Printf("并发分析第 %d/%d 张图片（URL直传: %s）", imageIndex, totalImages, url)

			detail := model.ImageAnalysisDetail{
				Index:    imageIndex,
				Source:   "url_download",
				ImageURL: url,
			}

			var extractedData *model.ExtractedData
			var requestId string
			var tokenUsage *model.TokenUsage
			var err error

			switch provider {
			case "qwen":
				extractedData, requestId, tokenUsage, err = s.qwenClient.ExtractDataFromImage(nil, url, employeeName, appData.ApplicationType, appData.ApplicationDate, appData.StartTime, appData.EndTime)
			case "volcano":
				extractedData, requestId, tokenUsage, err = s.volcanoClient.ExtractDataFromImage(nil, url, employeeName, appData.ApplicationType, appData.ApplicationDate, appData.StartTime, appData.EndTime)
			default:
				err = fmt.Errorf("未知的 AI provider: %s", provider)
			}

			// 设置requestId和tokenUsage
			detail.RequestId = requestId
			detail.TokenUsage = tokenUsage

			aiDuration := time.Since(aiStartTime)
			detail.ProcessingTimeMs = aiDuration.Milliseconds()

			if err != nil {
				detail.Success = false
				detail.ErrorMessage = err.Error()
				detail.IsValid = false
				log.Printf("✗ 第 %d 张图片分析失败 (耗时: %v): %v", imageIndex, aiDuration, err)
			} else {
				// 分析成功
				detail.Success = true
				detail.ExtractedData = extractedData
				detail.IsValid = extractedData.IsProofTypeValid
				log.Printf("第 %d 张图片分析完成 (耗时: %v): IsProofTypeValid=%v, ExtractedName=%s, RequestType=%s",
					imageIndex, aiDuration, extractedData.IsProofTypeValid, extractedData.ExtractedName, extractedData.RequestType)
			}

			resultChan <- analysisResult{
				detail:        detail,
				extractedData: extractedData,
				index:         len(fileHeaders) + index,
				err:           err,
			}
		}(i, imageURL)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	for result := range resultChan {
		imagesAnalysis = append(imagesAnalysis, result.detail)

		// 检查是否满足条件（证明材料类型有效）
		if validImageIndex == 0 && result.extractedData != nil && result.extractedData.IsProofTypeValid {
			validImageIndex = result.detail.Index
			log.Printf("✓ 第 %d 张图片满足条件", result.detail.Index)
		}
	}

	// 按索引排序结果
	for i := 0; i < len(imagesAnalysis)-1; i++ {
		for j := i + 1; j < len(imagesAnalysis); j++ {
			if imagesAnalysis[i].Index > imagesAnalysis[j].Index {
				imagesAnalysis[i], imagesAnalysis[j] = imagesAnalysis[j], imagesAnalysis[i]
			}
		}
	}

	// 6. 不再在此处提前生成简化原因，统一交由规则引擎产出详细失败原因

	// 7. 调用规则引擎进行最终裁决
	rulesStartTime := time.Now()
	log.Printf("开始规则引擎验证")

	// 构建所有图片的提取数据列表
	var allExtractedData []*model.ExtractedData
	for _, detail := range imagesAnalysis {
		if detail.Success && detail.ExtractedData != nil {
			allExtractedData = append(allExtractedData, detail.ExtractedData)
		}
	}

	result := rules.ValidateApplication(appData, nil, allExtractedData)
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

// GetEmployeeData 获取员工考勤数据
func (s *AnalysisService) GetEmployeeData(userId string) (*client.EmployeeData, error) {
	return s.oaClient.GetEmployeeData(userId)
}
