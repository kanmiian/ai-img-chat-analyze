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

	// 添加原始请求参数调试
	log.Printf("原始请求参数: %+v", c.Request.Form)

	// 添加multipart表单调试
	if form, err := c.MultipartForm(); err == nil && form != nil {
		log.Printf("MultipartForm Values: %+v", form.Value)
		log.Printf("MultipartForm Files: %+v", form.File)
	}

	// 1. 解析请求参数（支持JSON和表单两种格式）
	var reqData struct {
		UserId          string   `json:"user_id" form:"user_id"`
		Alias           string   `json:"alias" form:"alias"`
		ApplicationType string   `json:"application_type" form:"application_type" binding:"required"`
		ApplicationTime string   `json:"application_time" form:"application_time"` // 向后兼容
		StartTime       string   `json:"start_time" form:"start_time"`             // 上班时间
		EndTime         string   `json:"end_time" form:"end_time"`                 // 下班时间
		ApplicationDate string   `json:"application_date" form:"application_date"`
		Reason          string   `json:"reason" form:"reason"`
		ImageUrls       []string `json:"image_urls" form:"image_urls[]"`
		ImageBase64     string   `json:"image_base64" form:"image_base64"` // 新增：base64图片
		AttendanceInfo  []string `json:"attendance_info" form:"attendance_info[]"`
	}

	// 尝试JSON绑定，失败则尝试表单绑定
	if err := c.ShouldBindJSON(&reqData); err != nil {
		log.Printf("JSON绑定失败，尝试表单绑定: %v", err)
		if err := c.ShouldBind(&reqData); err != nil {
			log.Printf("测试请求参数绑定失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "请求参数无效",
				"error":   err.Error(),
			})
			return
		}
		log.Printf("使用表单格式解析成功")
	} else {
		log.Printf("使用JSON格式解析成功")
	}

	// 添加调试日志查看接收到的参数
	log.Printf("接收到的参数 - UserId: '%s', Alias: '%s', ApplicationType: '%s', StartTime: '%s', EndTime: '%s', ApplicationTime: '%s', ImageUrls长度: %d, ImageUrls内容: %v, ImageBase64长度: %d",
		reqData.UserId, reqData.Alias, reqData.ApplicationType, reqData.StartTime, reqData.EndTime, reqData.ApplicationTime, len(reqData.ImageUrls), reqData.ImageUrls, len(reqData.ImageBase64))

	// 2. 检查是否有图片（支持URL和base64两种方式）
	if len(reqData.ImageUrls) == 0 && reqData.ImageBase64 == "" {
		log.Printf("测试请求缺少图片（既无image_urls也无image_base64）")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "必须提供至少一张图片（image_urls或image_base64）",
		})
		return
	}

	// 3. 构建应用数据
	appData := model.ApplicationData{
		UserId:          reqData.UserId,
		Alias:           reqData.Alias,
		ApplicationType: reqData.ApplicationType,
		ApplicationTime: reqData.ApplicationTime, // 向后兼容
		StartTime:       reqData.StartTime,       // 上班时间
		EndTime:         reqData.EndTime,         // 下班时间
		ApplicationDate: reqData.ApplicationDate,
		Reason:          reqData.Reason,
		ImageUrls:       reqData.ImageUrls,
		AttendanceInfo:  reqData.AttendanceInfo,
	}

	// 如果有base64图片，将其转换为URL格式添加到ImageUrls中
	if reqData.ImageBase64 != "" {
		// 为base64图片生成一个临时的data URI
		dataURI := "data:image/jpeg;base64," + reqData.ImageBase64
		appData.ImageUrls = append(appData.ImageUrls, dataURI)
		log.Printf("添加base64图片到ImageUrls，当前ImageUrls长度: %d", len(appData.ImageUrls))
	}

	// 如果提供了新字段，优先使用新字段
	if reqData.StartTime != "" || reqData.EndTime != "" {
		log.Printf("使用新的时间字段 - StartTime: %s, EndTime: %s", reqData.StartTime, reqData.EndTime)
	} else if reqData.ApplicationTime != "" {
		log.Printf("使用向后兼容的时间字段 - ApplicationTime: %s", reqData.ApplicationTime)
	} else {
		log.Printf("警告: 未提供任何时间字段")
	}

	// 如果既没有传入alias，使用默认值
	if reqData.Alias == "" {
		// 如果既没有OA数据也没有传入alias，使用默认值
		log.Printf("未提供Alias，使用默认值")
		appData.Alias = "未知用户"
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

	// 6. 返回详细结果（与OA系统期望的格式一致）
	log.Printf("火山引擎测试完成 (总耗时: %v) - 结果: IsAbnormal=%v", totalDuration, result.IsAbnormal)

	c.JSON(http.StatusOK, gin.H{
		"success":     !result.IsAbnormal, // 通过 = 不异常
		"message":     result.Reason,      // 原因
		"is_abnormal": result.IsAbnormal,  // 直接返回，方便OA系统使用
		"reason":      result.Reason,      // 失败原因
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
