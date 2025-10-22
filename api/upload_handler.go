package api

import (
	"errors"
	"log"
	"my-ai-app/config"
	"my-ai-app/model"
	"my-ai-app/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	analysisService *service.AnalysisService
}

// NewUploadHandler 依赖注入 (不变)
func NewUploadHandler(cfg *config.Config) *UploadHandler {
	return &UploadHandler{
		analysisService: service.NewAnalysisService(cfg),
	}
}

// AnalyzeImage 已重命名为 AnalyzeApplication (核心修改)
func (h *UploadHandler) AnalyzeApplication(c *gin.Context) {
	// 1. 绑定表单数据 (如 "application_type", "application_time" 等)
	var appData model.ApplicationData
	if err := c.Bind(&appData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "表单数据无效", "details": err.Error()})
		return
	}

	// 2. 尝试获取图片文件 (这是可选的)
	fileHeader, err := c.FormFile("image")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		// 如果有错误，但不是 "没找到文件" 错误，说明是其他上传问题
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片上传失败", "details": err.Error()})
		return
	}

	// 如果 err == http.ErrMissingFile, 那么 fileHeader 会是 nil
	// 这正是我们想要的 "可选图片" 逻辑

	log.Printf("收到分析请求: %+v, 是否有图片: %v", appData, fileHeader != nil)

	// 3. 调用服务进行分析 (传入表单数据和(可能的)图片)
	result, err := h.analysisService.Analyze(appData, fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分析失败", "details": err.Error()})
		return
	}

	// 4. 成功返回
	c.JSON(http.StatusOK, result)
}
