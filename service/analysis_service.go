package service

import (
	"fmt"
	"log"
	"mime/multipart"
	"my-ai-app/client"
	"my-ai-app/config"
	"my-ai-app/model"
	"my-ai-app/rules"
	"strings"
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

	// 5. 并发处理多张图片
	var validExtractedData *model.ExtractedData
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
				extractedData, requestId, tokenUsage, err = s.qwenClient.ExtractDataFromImage(fh, "", employeeName, appData.ApplicationType, appData.ApplicationDate)
			case "volcano":
				extractedData, requestId, tokenUsage, err = s.volcanoClient.ExtractDataFromImage(fh, "", employeeName, appData.ApplicationType, appData.ApplicationDate)
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
				extractedData, requestId, tokenUsage, err = s.qwenClient.ExtractDataFromImage(nil, url, employeeName, appData.ApplicationType, appData.ApplicationDate)
			case "volcano":
				extractedData, requestId, tokenUsage, err = s.volcanoClient.ExtractDataFromImage(nil, url, employeeName, appData.ApplicationType, appData.ApplicationDate)
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
			validExtractedData = result.extractedData
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

		// 分析图片类型，提供具体的错误提示
		var imageTypes []string
		for _, detail := range imagesAnalysis {
			if detail.Success && detail.ExtractedData != nil {
				imageTypes = append(imageTypes, detail.ExtractedData.RequestType)
			}
		}

		// 根据申请类型提供具体的证明材料要求
		var reason string
		switch appData.ApplicationType {
		case "病假":
			requiredProof := "病历单、处方单、诊断证明"
			reason = fmt.Sprintf("提供的图片类型[%s]不符合%s申请要求，需要提供：%s",
				strings.Join(imageTypes, "、"), appData.ApplicationType, requiredProof)
		case "补打卡":
			// 特殊处理补打卡，强调时间问题
			reason = fmt.Sprintf("提供的图片均不能体现在%s的时间前已在公司上班", appData.ApplicationTime)
		case "事假":
			requiredProof := "相关证明文件"
			reason = fmt.Sprintf("提供的图片类型[%s]不符合%s申请要求，需要提供：%s",
				strings.Join(imageTypes, "、"), appData.ApplicationType, requiredProof)
		default:
			requiredProof := "相关证明材料"
			reason = fmt.Sprintf("提供的图片类型[%s]不符合%s申请要求，需要提供：%s",
				strings.Join(imageTypes, "、"), appData.ApplicationType, requiredProof)
		}

		return &model.AnalysisResult{
			IsAbnormal:     true,
			Reason:         reason,
			ImagesAnalysis: imagesAnalysis,
		}, nil
	}

	// 7. 尝试获取OA考勤数据（可选）
	var oaAttendanceData *model.OaAttendanceData
	if appData.UserId != "" && appData.ApplicationDate != "" {
		attendanceStartTime := time.Now()
		attendanceData, err := s.oaClient.GetAttendanceData(appData.UserId, appData.ApplicationDate)
		attendanceDuration := time.Since(attendanceStartTime)
		if err != nil {
			log.Printf("获取OA考勤数据失败 (耗时: %v): %v", attendanceDuration, err)
		} else {
			log.Printf("获取到OA考勤数据 (耗时: %v): %+v", attendanceDuration, attendanceData)
			// 转换为模型格式
			oaAttendanceData = &model.OaAttendanceData{
				Status:          attendanceData.AttendanceType,
				ClockInTime:     attendanceData.WorkStartTime,
				ClockOutTime:    attendanceData.WorkEndTime,
				StandardInTime:  "09:00", // 默认值，可以从配置或OA系统获取
				StandardOutTime: "18:00", // 默认值，可以从配置或OA系统获取
			}
		}
	}

	// 8. 调用规则引擎进行最终裁决
	rulesStartTime := time.Now()
	log.Printf("开始规则引擎验证")

	// 构建所有图片的提取数据列表
	var allExtractedData []*model.ExtractedData
	for _, detail := range imagesAnalysis {
		if detail.Success && detail.ExtractedData != nil {
			allExtractedData = append(allExtractedData, detail.ExtractedData)
		}
	}

	result := rules.ValidateApplication(appData, oaAttendanceData, allExtractedData)
	rulesDuration := time.Since(rulesStartTime)

	// 9. 添加详细分析结果
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
