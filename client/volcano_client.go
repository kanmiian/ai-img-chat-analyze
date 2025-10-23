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
	Model    string          `json:"model"`
	Messages []VisionMessage `json:"messages"`
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
func (c *VolcanoClient) ExtractDataFromImage(fileHeader *multipart.FileHeader, officialName string, appType string) (*model.ExtractedData, error) {

	// 1. (调用共享函数)
	base64Image, mimeType, err := imageToBase64(fileHeader)
	if err != nil {
		return nil, fmt.Errorf("图片转 Base64 失败: %w", err)
	}

	// 2. (调用共享函数)
	promptText := buildExtractorPrompt(officialName, appType)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	// 3. 构建请求体
	reqBody := VolcanoVisionRequest{
		Model: "doubao-seed-1-6-251015", // todo 火山模型
		Messages: []VisionMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: promptText},
					{Type: "image_url", ImageURL: &ChatMessageImageURL{URL: dataURI}},
				},
			},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建火山请求体失败: %w", err)
	}

	// 4. 创建 HTTP 请求
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("创建火山 HTTP 请求失败: %w", err)
	}

	// 5. 设置请求头 (火山使用 Bearer Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// 6. 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送火山 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 7. 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取火山响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("火山 API 请求失败，状态码: %d, 请求体: %s", resp.StatusCode, string(reqBytes))
		return nil, fmt.Errorf("火山 API 请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 8. 解析响应 (使用共享的 LlmResponse)
	var llmResp LlmResponse // <-- 复用 llm_shared.go 中的结构
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, fmt.Errorf("解析火山响应失败: %w, 响应: %s", err, string(respBody))
	}

	if llmResp.Error.Code != "" {
		return nil, fmt.Errorf("火山 API 错误: %s", llmResp.Error.Message)
	}

	if len(llmResp.Choices) == 0 || llmResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("火山 API 响应中没有找到有效内容, 响应: %s", string(respBody))
	}

	aiContent := llmResp.Choices[0].Message.Content
	aiContent = strings.TrimPrefix(aiContent, "```json")
	aiContent = strings.TrimSuffix(aiContent, "```")
	aiContent = strings.TrimSpace(aiContent)

	// 9. 将 AI 返回的 JSON 字符串解析为 ExtractedData
	var extractedData model.ExtractedData
	if err := json.Unmarshal([]byte(aiContent), &extractedData); err != nil {
		return nil, fmt.Errorf("解析 AI 返回的 JSON 内容失败: %w, AI内容: %s", err, aiContent)
	}

	return &extractedData, nil
}
