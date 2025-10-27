package api

import (
	"fmt"
	"log"
	"mime/multipart"
	"my-ai-app/config"
	"my-ai-app/model"
	"my-ai-app/service"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	analysisService *service.AnalysisService
}

func NewUploadHandler(cfg *config.Config) *UploadHandler {
	return &UploadHandler{
		analysisService: service.NewAnalysisService(cfg),
	}
}

// ---  Qwen ---
func (h *UploadHandler) AnalyzeQwen(c *gin.Context) {
	startTime := time.Now()
	log.Printf("收到Qwen分析请求 - IP: %s", c.ClientIP())

	appData, fileHeaders, err := h.bindRequest(c)
	if err != nil {
		log.Printf("Qwen请求绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求无效", "details": err.Error()})
		return
	}

	result, err := h.analysisService.AnalyzeWithQwen(appData, fileHeaders)
	totalDuration := time.Since(startTime)
	if err != nil {
		log.Printf("Qwen分析异常 (总耗时: %v): %v", totalDuration, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Qwen 分析失败", "details": err.Error()})
		return
	}

	log.Printf("Qwen分析完成 (总耗时: %v) - 结果: IsAbnormal=%v", totalDuration, result.IsAbnormal)
	c.JSON(http.StatusOK, result)
}

// --- Volcano ---
func (h *UploadHandler) AnalyzeVolcano(c *gin.Context) {
	startTime := time.Now()
	log.Printf("收到Volcano分析请求 - IP: %s", c.ClientIP())

	// 1. 调用私有助手解析请求
	appData, fileHeaders, err := h.bindRequest(c)
	if err != nil {
		log.Printf("Volcano请求绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求无效", "details": err.Error()})
		return
	}

	result, err := h.analysisService.AnalyzeWithVolcano(appData, fileHeaders)
	totalDuration := time.Since(startTime)
	if err != nil {
		log.Printf("Volcano分析异常 (总耗时: %v): %v", totalDuration, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Volcano 分析失败", "details": err.Error()})
		return
	}

	log.Printf("Volcano分析完成 (总耗时: %v) - 结果: IsAbnormal=%v", totalDuration, result.IsAbnormal)
	c.JSON(http.StatusOK, result)
}

// --- 火山引擎测试接口（简化版，供OA系统调用） ---
func (h *UploadHandler) TestVolcanoSimple(c *gin.Context) {
	startTime := time.Now()
	log.Printf("收到火山引擎测试请求 - IP: %s", c.ClientIP())

	// 1. 解析请求参数
	var reqData struct {
		UserId          string   `form:"user_id" binding:"required"`
		Alias           string   `form:"alias" binding:"required"`
		ApplicationType string   `form:"application_type" binding:"required"`
		ApplicationTime string   `form:"application_time"`
		ApplicationDate string   `form:"application_date"`
		Reason          string   `form:"reason"`
		ImageUrls       []string `form:"image_urls[]"`
	}

	if err := c.ShouldBind(&reqData); err != nil {
		log.Printf("测试请求参数绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数无效",
			"error":   err.Error(),
		})
		return
	}

	// 2. 检查是否有图片
	if len(reqData.ImageUrls) == 0 {
		log.Printf("测试请求缺少图片")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "必须提供至少一张图片",
		})
		return
	}

	// 3. 通过user_id获取员工考勤数据
	log.Printf("获取员工考勤数据 - UserId: %s", reqData.UserId)
	oaEmployeeData, err := h.analysisService.GetEmployeeData(reqData.UserId)
	if err != nil {
		log.Printf("获取员工考勤数据失败: %v", err)
		// 如果获取失败，使用传入的alias作为备选
		log.Printf("使用传入的alias作为备选: %s", reqData.Alias)
	}

	// 4. 构建应用数据
	appData := model.ApplicationData{
		UserId:          reqData.UserId,
		Alias:           reqData.Alias,
		ApplicationType: reqData.ApplicationType,
		ApplicationTime: reqData.ApplicationTime,
		ApplicationDate: reqData.ApplicationDate,
		Reason:          reqData.Reason,
		ImageUrls:       reqData.ImageUrls,
	}

	// 如果有考勤数据，使用考勤数据中的信息
	if oaEmployeeData != nil {
		log.Printf("使用OA考勤数据 - Alias: %s", oaEmployeeData.Alias)
		appData.Alias = oaEmployeeData.Alias
	}

	// 5. 调用火山引擎分析
	result, err := h.analysisService.AnalyzeWithVolcano(appData, nil)
	totalDuration := time.Since(startTime)

	if err != nil {
		log.Printf("火山引擎测试分析异常 (总耗时: %v): %v", totalDuration, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "分析失败",
			"error":   err.Error(),
		})
		return
	}

	// 6. 返回简化结果
	log.Printf("火山引擎测试完成 (总耗时: %v) - 结果: IsAbnormal=%v", totalDuration, result.IsAbnormal)

	c.JSON(http.StatusOK, gin.H{
		"success": !result.IsAbnormal, // 通过 = 不异常
		"message": result.Reason,      // 原因
		"data": gin.H{
			"is_abnormal":        result.IsAbnormal,
			"reason":             result.Reason,
			"valid_image_index":  result.ValidImageIndex,
			"total_images":       len(reqData.ImageUrls),
			"processing_time_ms": totalDuration.Milliseconds(),
		},
	})
}

// --- 私有助手: 解析表单数据和可选的图片（支持多图片） ---
func (h *UploadHandler) bindRequest(c *gin.Context) (model.ApplicationData, []*multipart.FileHeader, error) {
	var appData model.ApplicationData
	// 1. 绑定表单数据
	if err := c.Bind(&appData); err != nil {
		return model.ApplicationData{}, nil, fmt.Errorf("表单数据无效: %w", err)
	}

	// 2. 尝试获取多张图片
	form, err := c.MultipartForm()
	var fileHeaders []*multipart.FileHeader

	if err == nil && form != nil && form.File != nil {
		// 尝试获取 images[] 字段（多图片）
		if files, exists := form.File["images[]"]; exists && len(files) > 0 {
			fileHeaders = files
			log.Printf("收到 %d 张图片（images[]）", len(fileHeaders))
		} else if files, exists := form.File["images"]; exists && len(files) > 0 {
			// 兼容 images 字段
			fileHeaders = files
			log.Printf("收到 %d 张图片（images）", len(fileHeaders))
		} else if files, exists := form.File["image"]; exists && len(files) > 0 {
			// 兼容单图片上传 image 字段
			fileHeaders = files
			log.Printf("收到 %d 张图片（image）", len(fileHeaders))
		}
	}

	// 3. 合并 ImageUrl 和 ImageUrls
	if appData.ImageUrl != "" && len(appData.ImageUrls) == 0 {
		// 如果只有单个 ImageUrl，添加到 ImageUrls 数组
		appData.ImageUrls = []string{appData.ImageUrl}
	} else if appData.ImageUrl != "" && len(appData.ImageUrls) > 0 {
		// 如果两者都有，合并
		appData.ImageUrls = append([]string{appData.ImageUrl}, appData.ImageUrls...)
	}

	// 4. 检查图片和 URL 不能同时提供
	if len(fileHeaders) > 0 && len(appData.ImageUrls) > 0 {
		return appData, nil, fmt.Errorf("图片文件和图片URL不能同时上传")
	}

	log.Printf("解析完成 - 文件数: %d, URL数: %d", len(fileHeaders), len(appData.ImageUrls))
	return appData, fileHeaders, nil
}
