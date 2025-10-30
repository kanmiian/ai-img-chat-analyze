package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"my-ai-app/model"
	"net/http"
	"strings"
	"time"
)

// 提取最后一个完整的 JSON 对象字符串
func extractLastJSONObject(s string) string {
	s = strings.TrimSpace(s)
	// 快速路径：若本身就是以 { 开头并以 } 结尾，直接返回
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return s
	}
	// 去掉可能的围栏
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	// 扫描并收集所有平衡的 JSON 对象，取最后一个
	var last string
	depth := 0
	start := -1
	for i, r := range s {
		if r == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if r == '}' {
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					last = s[start : i+1]
					start = -1
				}
			}
		}
	}
	return strings.TrimSpace(last)
}

// VolcanoClient 结构体
type VolcanoClient struct {
	url        string
	apiKey     string
	httpClient *http.Client
}

// VolcanoVisionRequest 定义请求体 (OpenAI 兼容)
type VolcanoVisionRequest struct {
	Model       string          `json:"model"`
	Messages    []VisionMessage `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`      // 是否使用流式输出
	Temperature float64         `json:"temperature,omitempty"` // 温度参数
	TopP        float64         `json:"top_p,omitempty"`       // TopP参数
	MaxTokens   int             `json:"max_tokens,omitempty"`  // 最大token数
	Thinking    *ThinkingConfig `json:"thinking,omitempty"`    // 深度思考模式配置
}

// ThinkingConfig 深度思考模式配置
type ThinkingConfig struct {
	Type string `json:"type"` // "enabled" 或 "disabled"
}

// 为了在需要时安全地附加图片内容，这里提供一个别名和安全复制方法
type VisionMessageContentPartAlias ContentPart

func (cp *ContentPart) CopyOrZero() ContentPart {
	if cp == nil {
		return ContentPart{Type: "text", Text: ""}
	}
	return *cp
}

// NewVolcanoClient 创建火山客户端
func NewVolcanoClient(url string, apiKey string) *VolcanoClient {
	return &VolcanoClient{
		url:        url,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// ExtractDataFromImage 调用火山 API 提取图片数据
// 支持 fileHeader（直接上传）或 imageURL（URL直传）
// 返回：提取的数据、请求ID、Token使用情况、错误
func (c *VolcanoClient) ExtractDataFromImage(fileHeader *multipart.FileHeader, imageURL string, officialName string, appType string, applicationDate string, appStart string, appEnd string, needImageValidation bool, attendanceText string) (*model.ExtractedData, string, *model.TokenUsage, error) {
	startTime := time.Now()

	// 记录输入来源
	if fileHeader != nil {
		log.Printf("Volcano开始处理图片 - 来源: 文件上传, 文件名: %s, 大小: %d bytes, 姓名: %s, 类型: %s",
			fileHeader.Filename, fileHeader.Size, officialName, appType)
	} else if imageURL != "" {
		log.Printf("Volcano开始处理图片 - 来源: URL直传, URL: %s, 姓名: %s, 类型: %s",
			imageURL, officialName, appType)
	}

	// 1. 构建图片内容（base64 或 URL），当需要图片核验时
	var imageContent *VisionMessageContentPartAlias
	var imageDuration time.Duration
	var err error
	if needImageValidation {
		imageStartTime := time.Now()
		var ic *ContentPart
		ic, err = buildImageContentPart(fileHeader, imageURL)
		imageDuration = time.Since(imageStartTime)
		if err != nil {
			log.Printf("图片处理失败 (耗时: %v): %v", imageDuration, err)
			return nil, "", nil, fmt.Errorf("图片处理失败: %w", err)
		}
		log.Printf("图片内容构建完成 (耗时: %v)", imageDuration)
		// 为了统一类型，使用别名承接后续组装
		imageContent = (*VisionMessageContentPartAlias)(ic)
	}

	// 2. 构建prompt（区分是否需要图片核验）
	var promptText string
	if needImageValidation {
		promptText = buildPromptByType(officialName, appType, applicationDate, displayAppTime(appStart, appEnd))
		if ic := (*ContentPart)(imageContent); ic != nil && ic.ImageURL != nil {
			proof := ic.ImageURL.URL
			promptText = strings.ReplaceAll(promptText, "{{IMAGE_PROOF}}", proof)
		}
		promptText = strings.ReplaceAll(promptText, "{{APPLICATION_DATE}}", applicationDate)
		promptText = strings.ReplaceAll(promptText, "{{APPLICATION_TIME}}", displayAppTime(appStart, appEnd))
		promptText = strings.ReplaceAll(promptText, "{{APPLICATION_TYPE}}", appType)
		promptText = strings.ReplaceAll(promptText, "{{EMPLOYEE_NAME}}", officialName)
	} else {
		promptText = buildNoImagePrompt(officialName, appType, applicationDate, displayAppTime(appStart, appEnd), attendanceText)
	}
	log.Printf("火山prompt: %s", promptText)
	// 3. 构建请求体
	var messages []VisionMessage
	if needImageValidation {
		messages = []VisionMessage{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: promptText}, (*ContentPart)(imageContent).CopyOrZero()}},
		}
	} else {
		messages = []VisionMessage{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: promptText}}},
		}
	}
	reqBody := VolcanoVisionRequest{
		Model:       "doubao-seed-1-6-lite-251015",
		Messages:    messages,
		Stream:      false,
		Temperature: 0.1,
		Thinking:    &ThinkingConfig{Type: "disabled"},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", nil, fmt.Errorf("构建火山请求体失败: %w", err)
	}

	// 4. 创建 HTTP 请求
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, "", nil, fmt.Errorf("创建火山 HTTP 请求失败: %w", err)
	}

	// 5. 设置请求头 (火山使用 Bearer Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// 6. 发送请求
	httpStartTime := time.Now()
	log.Printf("发送Volcano HTTP请求 - URL: %s, 请求体大小: %d bytes", c.url, len(reqBytes))
	resp, err := c.httpClient.Do(req)
	httpDuration := time.Since(httpStartTime)
	if err != nil {
		log.Printf("Volcano HTTP请求失败 (耗时: %v): %v", httpDuration, err)
		return nil, "", nil, fmt.Errorf("发送火山 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("Volcano HTTP请求完成 (耗时: %v, 状态码: %d)", httpDuration, resp.StatusCode)

	// 7. 读取响应
	readStartTime := time.Now()
	respBody, err := io.ReadAll(resp.Body)
	readDuration := time.Since(readStartTime)
	if err != nil {
		log.Printf("读取Volcano响应失败 (耗时: %v): %v", readDuration, err)
		return nil, "", nil, fmt.Errorf("读取火山响应体失败: %w", err)
	}
	log.Printf("读取Volcano响应完成 (耗时: %v, 响应大小: %d bytes)", readDuration, len(respBody))

	if resp.StatusCode != http.StatusOK {
		log.Printf("火山 API 请求失败，状态码: %d, 请求体: %s", resp.StatusCode, string(reqBytes))
		return nil, "", nil, fmt.Errorf("火山 API 请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 8. 解析响应 (使用共享的 LlmResponse)
	var llmResp LlmResponse // <-- 复用 llm_shared.go 中的结构
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, "", nil, fmt.Errorf("解析火山响应失败: %w, 响应: %s", err, string(respBody))
	}
	log.Printf("火山响应: %s", string(respBody))
	// 9. 提取requestId和tokenUsage
	requestId := llmResp.Id
	var tokenUsage *model.TokenUsage
	if llmResp.Usage != nil {
		tokenUsage = &model.TokenUsage{
			CompletionTokens: llmResp.Usage.CompletionTokens,
			PromptTokens:     llmResp.Usage.PromptTokens,
			TotalTokens:      llmResp.Usage.TotalTokens,
		}
		log.Printf("Volcano请求ID: %s, Token使用: prompt=%d, completion=%d, total=%d",
			requestId, tokenUsage.PromptTokens, tokenUsage.CompletionTokens, tokenUsage.TotalTokens)
	} else {
		log.Printf("Volcano请求ID: %s (未返回token使用信息)", requestId)
	}

	if llmResp.Error.Code != "" {
		return nil, requestId, tokenUsage, fmt.Errorf("火山 API 错误: %s", llmResp.Error.Message)
	}

	if len(llmResp.Choices) == 0 || llmResp.Choices[0].Message.Content == "" {
		return nil, requestId, tokenUsage, fmt.Errorf("火山 API 响应中没有找到有效内容, 响应: %s", string(respBody))
	}

	aiContent := llmResp.Choices[0].Message.Content
	aiContent = strings.TrimPrefix(aiContent, "```json")
	aiContent = strings.TrimSuffix(aiContent, "```")
	aiContent = strings.TrimSpace(aiContent)

	// 仅提取并解析最后一个 JSON 对象，确保只返回一组结果
	jsonToParse := extractLastJSONObject(aiContent)
	if jsonToParse == "" {
		return nil, requestId, tokenUsage, fmt.Errorf("未找到有效的 JSON 对象, AI内容: %s", aiContent)
	}

	// 10. 将 AI 返回的 JSON 字符串解析为 ExtractedData
	parseStartTime := time.Now()
	var extractedData model.ExtractedData
	if err := json.Unmarshal([]byte(jsonToParse), &extractedData); err != nil {
		log.Printf("解析AI返回JSON失败 (耗时: %v): %v, 内容: %s", time.Since(parseStartTime), err, jsonToParse)
		return nil, requestId, tokenUsage, fmt.Errorf("解析 AI 返回的 JSON 内容失败: %w, JSON: %s", err, jsonToParse)
	}
	parseDuration := time.Since(parseStartTime)

	// 额外字段映射与兜底：打卡类型等
	if extractedData.RequestType == "" {
		extractedData.RequestType = appType
	}
	if extractedData.RequestDate == "" {
		extractedData.RequestDate = applicationDate
	}
	if extractedData.RequestTime == "" {
		extractedData.RequestTime = displayAppTime(appStart, appEnd)
	}
	if extractedData.ExtractedName == "" {
		extractedData.ExtractedName = officialName
	}
	// 同步有效性判定
	if extractedData.Approve {
		extractedData.IsValid = true
		extractedData.IsProofTypeValid = true
	}

	totalDuration := time.Since(startTime)
	log.Printf("Volcano处理完成 - RequestId: %s, 总耗时: %v (图片处理: %v, HTTP: %v, 读取: %v, 解析: %v)",
		requestId, totalDuration, imageDuration, httpDuration, readDuration, parseDuration)

	return &extractedData, requestId, tokenUsage, nil
}
