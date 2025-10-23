package api

import (
	"errors"
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

	appData, fileHeader, err := h.bindRequest(c)
	if err != nil {
		log.Printf("Qwen请求绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求无效", "details": err.Error()})
		return
	}

	result, err := h.analysisService.AnalyzeWithQwen(appData, fileHeader)
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
	appData, fileHeader, err := h.bindRequest(c)
	if err != nil {
		log.Printf("Volcano请求绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求无效", "details": err.Error()})
		return
	}

	result, err := h.analysisService.AnalyzeWithVolcano(appData, fileHeader)
	totalDuration := time.Since(startTime)
	if err != nil {
		log.Printf("Volcano分析异常 (总耗时: %v): %v", totalDuration, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Volcano 分析失败", "details": err.Error()})
		return
	}

	log.Printf("Volcano分析完成 (总耗时: %v) - 结果: IsAbnormal=%v", totalDuration, result.IsAbnormal)
	c.JSON(http.StatusOK, result)
}

// --- 私有助手: 解析表单数据和可选的图片 ---
func (h *UploadHandler) bindRequest(c *gin.Context) (model.ApplicationData, *multipart.FileHeader, error) {
	var appData model.ApplicationData
	// 1. 绑定表单数据
	if err := c.Bind(&appData); err != nil {
		return model.ApplicationData{}, nil, fmt.Errorf("表单数据无效: %w", err)
	}

	// 2. 尝试获取图片 (可选)
	fileHeader, err := c.FormFile("image")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		// 如果有错误，但不是 "没找到文件" 错误，说明是其他上传问题
		return appData, nil, fmt.Errorf("图片上传失败: %w", err)
	}

	// 无论 err 是 nil (有图) 还是 ErrMissingFile (无图)，都返回
	return appData, fileHeader, nil
}
