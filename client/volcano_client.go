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
func (c *VolcanoClient) ExtractDataFromImage(fileHeader *multipart.FileHeader, imageURL string, officialName string, appType string, applicationDate string, appStart string, appEnd string) (*model.ExtractedData, string, *model.TokenUsage, error) {
	startTime := time.Now()

	// 记录输入来源
	if fileHeader != nil {
		log.Printf("Volcano开始处理图片 - 来源: 文件上传, 文件名: %s, 大小: %d bytes, 姓名: %s, 类型: %s",
			fileHeader.Filename, fileHeader.Size, officialName, appType)
	} else if imageURL != "" {
		log.Printf("Volcano开始处理图片 - 来源: URL直传, URL: %s, 姓名: %s, 类型: %s",
			imageURL, officialName, appType)
	}

	// 1. 构建图片内容（base64 或 URL）
	imageStartTime := time.Now()
	imageContent, err := buildImageContentPart(fileHeader, imageURL)
	imageDuration := time.Since(imageStartTime)
	if err != nil {
		log.Printf("图片处理失败 (耗时: %v): %v", imageDuration, err)
		return nil, "", nil, fmt.Errorf("图片处理失败: %w", err)
	}
	log.Printf("图片内容构建完成 (耗时: %v)", imageDuration)

	// 2. 构建prompt
	promptText := buildExtractorPrompt(officialName, appType, applicationDate, appStart, appEnd)
	log.Printf("火山prompt: %s", promptText)
	// 3. 构建请求体
	reqBody := VolcanoVisionRequest{
		Model: "doubao-seed-1-6-vision-250815", // todo 火山模型
		Messages: []VisionMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: promptText},
					*imageContent, // 使用构建的图片内容
				},
			},
		},
		Stream:      false, // 不使用流式输出
		Temperature: 0.1,   // 低温度，提高准确性
		Thinking: &ThinkingConfig{
			Type: "disabled", // 禁用深度思考模式
		},
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

	// 10. 将 AI 返回的 JSON 字符串解析为 ExtractedData
	parseStartTime := time.Now()
	var extractedData model.ExtractedData
	if err := json.Unmarshal([]byte(aiContent), &extractedData); err != nil {
		log.Printf("解析AI返回JSON失败 (耗时: %v): %v, 内容: %s", time.Since(parseStartTime), err, aiContent)
		return nil, requestId, tokenUsage, fmt.Errorf("解析 AI 返回的 JSON 内容失败: %w, AI内容: %s", err, aiContent)
	}
	parseDuration := time.Since(parseStartTime)

	totalDuration := time.Since(startTime)
	log.Printf("Volcano处理完成 - RequestId: %s, 总耗时: %v (图片处理: %v, HTTP: %v, 读取: %v, 解析: %v)",
		requestId, totalDuration, imageDuration, httpDuration, readDuration, parseDuration)

	return &extractedData, requestId, tokenUsage, nil
}

// CheckByNoImage 基于申请参数和考勤信息进行文本分析（无需图片）
// 返回：分析结果、请求ID、Token使用情况、错误
func (c *VolcanoClient) CheckByNoImage(appType string, appName string, appDate string, appStart string, appEnd string, attendanceInfo []string) (map[string]interface{}, string, *model.TokenUsage, error) {
	startTime := time.Now()
	log.Printf("Volcano开始文本分析 - 申请类型: %s, 员工: %s, 日期: %s", appType, appName, appDate)

	// 1. 构建文本prompt
	promptText := buildCheckByNoImagePrompt(appType, appName, appDate, appStart, appEnd, attendanceInfo)
	log.Printf("火山文本prompt: %s", promptText)

	// 2. 构建请求体（纯文本，无图片）
	reqBody := VolcanoVisionRequest{
		Model: "doubao-seed-1-6-vision-250815", // 使用相同的模型
		Messages: []VisionMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: promptText},
				},
			},
		},
		Stream:      false,
		Temperature: 0.1,
		Thinking: &ThinkingConfig{
			Type: "disabled",
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", nil, fmt.Errorf("构建火山文本请求体失败: %w", err)
	}

	// 3. 创建 HTTP 请求
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, "", nil, fmt.Errorf("创建火山文本 HTTP 请求失败: %w", err)
	}

	// 4. 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// 5. 发送请求
	httpStartTime := time.Now()
	log.Printf("发送Volcano文本HTTP请求 - URL: %s, 请求体大小: %d bytes", c.url, len(reqBytes))
	resp, err := c.httpClient.Do(req)
	httpDuration := time.Since(httpStartTime)
	if err != nil {
		log.Printf("Volcano文本HTTP请求失败 (耗时: %v): %v", httpDuration, err)
		return nil, "", nil, fmt.Errorf("发送火山文本 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("Volcano文本HTTP请求完成 (耗时: %v, 状态码: %d)", httpDuration, resp.StatusCode)

	// 6. 读取响应
	readStartTime := time.Now()
	respBody, err := io.ReadAll(resp.Body)
	readDuration := time.Since(readStartTime)
	if err != nil {
		log.Printf("读取Volcano文本响应失败 (耗时: %v): %v", readDuration, err)
		return nil, "", nil, fmt.Errorf("读取火山文本响应体失败: %w", err)
	}
	log.Printf("读取Volcano文本响应完成 (耗时: %v, 响应大小: %d bytes)", readDuration, len(respBody))

	if resp.StatusCode != http.StatusOK {
		log.Printf("火山文本 API 请求失败，状态码: %d, 请求体: %s", resp.StatusCode, string(reqBytes))
		return nil, "", nil, fmt.Errorf("火山文本 API 请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 7. 解析响应
	var llmResp LlmResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, "", nil, fmt.Errorf("解析火山文本响应失败: %w, 响应: %s", err, string(respBody))
	}
	log.Printf("火山文本响应: %s", string(respBody))

	// 8. 提取requestId和tokenUsage
	requestId := llmResp.Id
	var tokenUsage *model.TokenUsage
	if llmResp.Usage != nil {
		tokenUsage = &model.TokenUsage{
			CompletionTokens: llmResp.Usage.CompletionTokens,
			PromptTokens:     llmResp.Usage.PromptTokens,
			TotalTokens:      llmResp.Usage.TotalTokens,
		}
		log.Printf("Volcano文本请求ID: %s, Token使用: prompt=%d, completion=%d, total=%d",
			requestId, tokenUsage.PromptTokens, tokenUsage.CompletionTokens, tokenUsage.TotalTokens)
	} else {
		log.Printf("Volcano文本请求ID: %s (未返回token使用信息)", requestId)
	}

	if llmResp.Error.Code != "" {
		return nil, requestId, tokenUsage, fmt.Errorf("火山文本 API 错误: %s", llmResp.Error.Message)
	}

	if len(llmResp.Choices) == 0 || llmResp.Choices[0].Message.Content == "" {
		return nil, requestId, tokenUsage, fmt.Errorf("火山文本 API 响应中没有找到有效内容, 响应: %s", string(respBody))
	}

	// 9. 解析AI返回的JSON
	aiContent := llmResp.Choices[0].Message.Content
	aiContent = strings.TrimPrefix(aiContent, "```json")
	aiContent = strings.TrimSuffix(aiContent, "```")
	aiContent = strings.TrimSpace(aiContent)

	parseStartTime := time.Now()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(aiContent), &result); err != nil {
		log.Printf("解析AI返回JSON失败 (耗时: %v): %v, 内容: %s", time.Since(parseStartTime), err, aiContent)
		return nil, requestId, tokenUsage, fmt.Errorf("解析 AI 返回的 JSON 内容失败: %w, AI内容: %s", err, aiContent)
	}
	parseDuration := time.Since(parseStartTime)

	totalDuration := time.Since(startTime)
	log.Printf("Volcano文本处理完成 - RequestId: %s, 总耗时: %v (HTTP: %v, 读取: %v, 解析: %v)",
		requestId, totalDuration, httpDuration, readDuration, parseDuration)

	return result, requestId, tokenUsage, nil
}

// CheckByWithImageAuth 根据need_image_auth参数决定是否进行图片校验
// need_image_auth为true时调用ExtractDataFromImage，为false时调用CheckByNoImage
func (c *VolcanoClient) CheckByWithImageAuth(needImageAuth bool, fileHeader *multipart.FileHeader, imageURL string, appType string, appName string, appDate string, appStart string, appEnd string, attendanceInfo []string) (interface{}, string, *model.TokenUsage, error) {
	if needImageAuth {
		// 需要图片校验，调用原有的图片分析方法
		return c.ExtractDataFromImage(fileHeader, imageURL, appName, appType, appDate, appStart, appEnd)
	} else {
		// 不需要图片校验，调用文本分析方法
		return c.CheckByNoImage(appType, appName, appDate, appStart, appEnd, attendanceInfo)
	}
}