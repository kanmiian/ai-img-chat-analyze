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

// QwenClient 结构体
type QwenClient struct {
	url        string
	apiKey     string
	httpClient *http.Client
}

// QwenVisionRequest 定义 Qwen API 的请求体
type QwenVisionRequest struct {
	Model     string                 `json:"model"`
	Messages  []VisionMessage        `json:"messages"`
	ExtraBody map[string]interface{} `json:"extra_body,omitempty"` // <-- Qwen 特有
}

// NewQwenClient 创建一个新的 Qwen 客户端
func NewQwenClient(url string, apiKey string) *QwenClient {
	return &QwenClient{
		url:        url,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// ExtractDataFromImage 调用 Qwen API 提取图片数据
func (c *QwenClient) ExtractDataFromImage(fileHeader *multipart.FileHeader, officialName string, appType string) (*model.ExtractedData, error) {
	startTime := time.Now()
	log.Printf("Qwen开始处理图片 - 文件大小: %d bytes, 姓名: %s, 类型: %s",
		fileHeader.Size, officialName, appType)

	// 1. (调用共享函数)
	base64StartTime := time.Now()
	base64Image, mimeType, err := imageToBase64(fileHeader)
	base64Duration := time.Since(base64StartTime)
	if err != nil {
		log.Printf("图片转Base64失败 (耗时: %v): %v", base64Duration, err)
		return nil, fmt.Errorf("图片转 Base64 失败: %w", err)
	}
	log.Printf("图片转Base64完成 (耗时: %v, 大小: %d chars)", base64Duration, len(base64Image))

	// 2. (调用共享函数)
	promptText := buildExtractorPrompt(officialName, appType)

	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	// 3. 构建请求体 (!! 使用 Qwen 特有结构 !!)
	reqBody := QwenVisionRequest{
		Model: "qwen3-vl-plus", // <-- 使用 Qwen 模型 ID
		Messages: []VisionMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: promptText},
					{Type: "image_url", ImageURL: &ChatMessageImageURL{URL: dataURI}},
				},
			},
		},
		ExtraBody: map[string]interface{}{ // <-- Qwen 特有参数
			"enable_thinking": true,
			"thinking_budget": 81920,
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建 Qwen 请求体失败: %w", err)
	}

	// 4. 创建 HTTP 请求
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("创建 Qwen HTTP 请求失败: %w", err)
	}

	// 5. 设置请求头 (!! Qwen 使用 Bearer Token !!)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey) // <-- 保持一致

	// 6. 发送请求
	httpStartTime := time.Now()
	log.Printf("发送Qwen HTTP请求 - URL: %s, 请求体大小: %d bytes", c.url, len(reqBytes))
	resp, err := c.httpClient.Do(req)
	httpDuration := time.Since(httpStartTime)
	if err != nil {
		log.Printf("Qwen HTTP请求失败 (耗时: %v): %v", httpDuration, err)
		return nil, fmt.Errorf("发送 Qwen HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("Qwen HTTP请求完成 (耗时: %v, 状态码: %d)", httpDuration, resp.StatusCode)

	// 7. 读取响应
	readStartTime := time.Now()
	respBody, err := io.ReadAll(resp.Body)
	readDuration := time.Since(readStartTime)
	if err != nil {
		log.Printf("读取Qwen响应失败 (耗时: %v): %v", readDuration, err)
		return nil, fmt.Errorf("读取 Qwen 响应体失败: %w", err)
	}
	log.Printf("读取Qwen响应完成 (耗时: %v, 响应大小: %d bytes)", readDuration, len(respBody))

	if resp.StatusCode != http.StatusOK {
		log.Printf("Qwen API 请求失败，状态码: %d, 请求体: %s", resp.StatusCode, string(reqBytes))
		return nil, fmt.Errorf("Qwen API 请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 8. 解析响应 (使用共享的 LlmResponse 结构)
	var llmResp LlmResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, fmt.Errorf("解析 Qwen 响应失败: %w, 响应: %s", err, string(respBody))
	}

	if llmResp.Error.Code != "" {
		return nil, fmt.Errorf("Qwen API 错误: %s", llmResp.Error.Message)
	}

	if len(llmResp.Choices) == 0 || llmResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("Qwen API 响应中没有找到有效内容, 响应: %s", string(respBody))
	}

	aiContent := llmResp.Choices[0].Message.Content
	aiContent = strings.TrimPrefix(aiContent, "```json")
	aiContent = strings.TrimSuffix(aiContent, "```")
	aiContent = strings.TrimSpace(aiContent)

	// 9. 将 AI 返回的 JSON 字符串解析为 ExtractedData
	parseStartTime := time.Now()
	var extractedData model.ExtractedData
	if err := json.Unmarshal([]byte(aiContent), &extractedData); err != nil {
		log.Printf("解析AI返回JSON失败 (耗时: %v): %v, 内容: %s", time.Since(parseStartTime), err, aiContent)
		return nil, fmt.Errorf("解析 AI 返回的 JSON 内容失败: %w, AI内容: %s", err, aiContent)
	}
	parseDuration := time.Since(parseStartTime)

	totalDuration := time.Since(startTime)
	log.Printf("Qwen处理完成 - 总耗时: %v (Base64: %v, HTTP: %v, 读取: %v, 解析: %v)",
		totalDuration, base64Duration, httpDuration, readDuration, parseDuration)

	return &extractedData, nil
}
