package client

import (
	"bytes"
	"encoding/base64"
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

// VolcanoClient 结构体 (保持不变)
type VolcanoClient struct {
	url        string
	apiKey     string
	httpClient *http.Client
}

// NewVolcanoClient (保持不变)
func NewVolcanoClient(url string, apiKey string) *VolcanoClient {
	return &VolcanoClient{
		url:        url,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// ----------------------------------------------------
// 以下是新的请求/响应结构体 (!! 必须按火山图文 API 文档修改 !!)
// ----------------------------------------------------

// VolcanoVisionRequest 定义图文理解 API 的请求体
type VolcanoVisionRequest struct {
	Model    string          `json:"model"` // 例如: "doubao-pro-vision"
	Messages []VisionMessage `json:"messages"`
	// Parameters map[string]interface{} `json:"parameters,omitempty"` // 可选参数
}

// VisionMessage 多模态消息
type VisionMessage struct {
	Role    string        `json:"role"`    // "user"
	Content []ContentPart `json:"content"` // 内容是一个列表
}

// 结构与火山 SDK model.ChatCompletionMessageContentPart 保持一致
type ContentPart struct {
	Type     string               `json:"type"`                // "text" 或 "image_url"
	Text     string               `json:"text,omitempty"`      // type="text" 时使用
	ImageURL *ChatMessageImageURL `json:"image_url,omitempty"` // type="image_url" 时使用
}

// ChatMessageImageURL 图片 URL (新的)
// 结构与火山 SDK model.ChatMessageImageURL 保持一致
type ChatMessageImageURL struct {
	URL string `json:"url"` // "data:image/jpeg;base64,..."
}

// VolcanoResponse (与之前相同，假设返回结构一致)
type VolcanoResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ----------------------------------------------------
// 新的 InferFromImage 方法 (原 Infer 方法可删除)
// ----------------------------------------------------

// InferFromImage 调用火山引擎图文 API 进行推理
func (c *VolcanoClient) InferFromImage(fileHeader *multipart.FileHeader) (*model.AnalysisResult, error) {

	// 1. 将图片文件转为 Base64 和 MimeType
	base64Image, mimeType, err := imageToBase64(fileHeader) // <-- 修改
	if err != nil {
		return nil, fmt.Errorf("图片转 Base64 失败: %w", err)
	}

	// 2. 构建 Prompt
	promptText := buildVisionPrompt()

	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	reqBody := VolcanoVisionRequest{
		Model: "doubao-seed-1-6-251015", // !! 请再次确认您的模型 ID
		Messages: []VisionMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "text",
						Text: promptText,
					},
					{
						Type: "image_url", // <-- 修正 Type
						ImageURL: &ChatMessageImageURL{ // <-- 修正字段
							URL: dataURI, // <-- 传入 Data URI
						},
					},
				},
			},
		},
	}

	// 3. 序列化请求体
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建火山请求体失败: %w", err)
	}

	// 4. 创建 HTTP 请求
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("创建火山 HTTP 请求失败: %w", err)
	}

	// 5. 设置请求头
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

	// 8. 解析响应
	var volcanoResp VolcanoResponse
	if err := json.Unmarshal(respBody, &volcanoResp); err != nil {
		return nil, fmt.Errorf("解析火山响应失败: %w, 响应: %s", err, string(respBody))
	}

	if volcanoResp.Error.Code != "" {
		return nil, fmt.Errorf("火山 API 错误: %s", volcanoResp.Error.Message)
	}

	if len(volcanoResp.Choices) == 0 || volcanoResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("火山 API 响应中没有找到有效内容, 响应: %s", string(respBody))
	}

	// 9. 处理 AI 返回的内容
	aiContent := volcanoResp.Choices[0].Message.Content
	aiContent = strings.TrimPrefix(aiContent, "```json")
	aiContent = strings.TrimSuffix(aiContent, "```")
	aiContent = strings.TrimSpace(aiContent)

	// 10. 将 AI 返回的 JSON 字符串解析为标准 AnalysisResult 结构体
	var finalResult model.AnalysisResult
	if err := json.Unmarshal([]byte(aiContent), &finalResult); err != nil {
		return nil, fmt.Errorf("解析 AI 返回的 JSON 内容失败: %w, AI内容: %s", err, aiContent)
	}

	return &finalResult, nil
}

// ExtractDataFromImage 调用火山 AI 提取图片数据
func (c *VolcanoClient) ExtractDataFromImage(fileHeader *multipart.FileHeader, officialName string) (*model.ExtractedData, error) {

	base64Image, mimeType, err := imageToBase64(fileHeader)
	if err != nil {
		return nil, fmt.Errorf("图片转 Base64 失败: %w", err)
	}

	// 1. 构建新的 "提取器" Prompt
	promptText := buildExtractorPrompt(officialName)

	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	reqBody := VolcanoVisionRequest{
		Model: "doubao-seed-1-6-251015", // (模型 ID 保持不变)
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

	// 2. 序列化请求体
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建火山请求体失败: %w", err)
	}

	// 3. 创建 HTTP 请求
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("创建火山 HTTP 请求失败: %w", err)
	}

	// 4. 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// 5. 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送火山 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 6. 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取火山响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("火山 API 请求失败，状态码: %d, 请求体: %s", resp.StatusCode, string(reqBytes))
		return nil, fmt.Errorf("火山 API 请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 7. 解析响应
	var volcanoResp VolcanoResponse
	if err := json.Unmarshal(respBody, &volcanoResp); err != nil {
		return nil, fmt.Errorf("解析火山响应失败: %w, 响应: %s", err, string(respBody))
	}

	if volcanoResp.Error.Code != "" {
		return nil, fmt.Errorf("火山 API 错误: %s", volcanoResp.Error.Message)
	}

	if len(volcanoResp.Choices) == 0 || volcanoResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("火山 API 响应中没有找到有效内容, 响应: %s", string(respBody))
	}

	aiContent := volcanoResp.Choices[0].Message.Content
	aiContent = strings.TrimPrefix(aiContent, "```json")
	aiContent = strings.TrimSuffix(aiContent, "```")
	aiContent = strings.TrimSpace(aiContent)

	var extractedData model.ExtractedData
	if err := json.Unmarshal([]byte(aiContent), &extractedData); err != nil {
		return nil, fmt.Errorf("解析 AI 返回的 JSON 内容失败: %w, AI内容: %s", err, aiContent)
	}

	return &extractedData, nil
}

// ----------------------------------------------------
// 辅助函数
// ----------------------------------------------------

// buildVisionPrompt 用于分析异常判断的 Prompt
func buildVisionPrompt() string {
	return `
你是一个专业的人力资源（HR）助理。你的任务是分析这张图片中的员工申请单，提取关键信息，并判断是否异常。

**规则**:
1.  上班时间是 09:00。早于或等于 09:00 的补打卡被视为正常。晚于 09:00 的补打卡被视为异常。
2.  从图片中提取信息：员工姓名 (employee_name), 申请日期 (request_date), 申请时间 (request_time), 申请类型 (request_type)。
3.  如果信息不完整，将对应字段的值设为 "未知"。
4.  必须严格按照下面的 JSON 格式返回，不要包含任何额外的解释或 Markdown 代码块标记。

**JSON格式**:
{
  "is_abnormal": <true_or_false>,
  "reason": "<分析出的异常原因，正常则为'正常'>",
  "data": {
    "employee_name": "<提取的姓名>",
    "request_date": "<提取的日期 yyyy-MM-dd>",
    "request_time": "<提取的时间 HH:mm>",
    "request_type": "<提取的类型>"
  }
}
`
}

// buildExtractorPrompt 用于数据提取的 Prompt
func buildExtractorPrompt(officialName string) string {
	if officialName == "" {
		// 没有目标姓名时，让 AI 自行识别图片中的所有姓名
		return `
你是一个数据提取助理。你的任务是分析这张图片，它可能是病历单、浏览器记录、系统使用时间截图等。
请从中提取关键信息。

**规则**:
1.  尽力提取图片中的日期 (request_date)，格式 "yyyy-MM-dd"。
2.  尽力提取图片中的时间 (request_time)，格式 "HH:mm"。
3.  判断图片类型 (request_type)，例如："病历单", "浏览器记录", "系统截图", "其他"。
4.  提取图片中的关键文字 (content)，例如病历单的诊断结果。
5.  提取图片中出现的所有姓名 (extracted_name)，如果找到多个姓名，用逗号分隔。
6.  如果信息不存在，将对应字段的值设为 "未知"。
7.  必须严格按照下面的 JSON 格式返回，不要包含任何额外的解释。

**JSON格式**:
{
  "extracted_name": "<图片上找到的姓名，找不到则为'未知'>",
  "request_date": "<图片上的日期>",
  "request_time": "<图片上的时间>",
  "request_type": "<判断出的图片类型>",
  "content": "<图片中的关键文字摘要>"
}
`
	}

	// 有目标姓名时，指导 AI 重点识别该姓名
	return fmt.Sprintf(`
你是一个数据提取助理。你的任务是分析这张图片，它可能是病历单、浏览器记录、系统使用时间截图等。
请从中提取关键信息。
**目标员工姓名是：%s**。请重点识别图片中是否包含此姓名。

**规则**:
1.  尽力提取图片中的日期 (request_date)，格式 "yyyy-MM-dd"。
2.  尽力提取图片中的时间 (request_time)，格式 "HH:mm"。
3.  判断图片类型 (request_type)，例如："病历单", "浏览器记录", "系统截图", "其他"。
4.  提取图片中的关键文字 (content)，例如病历单的诊断结果。
5.  重点识别图片中是否包含目标姓名 "%s"，如果找到则填入 extracted_name，否则为"未知"。
6.  如果信息不存在，将对应字段的值设为 "未知"。
7.  必须严格按照下面的 JSON 格式返回，不要包含任何额外的解释。

**JSON格式**:
{
  "extracted_name": "<图片上找到的姓名，找不到则为'未知'>",
  "request_date": "<图片上的日期>",
  "request_time": "<图片上的时间>",
  "request_type": "<判断出的图片类型>",
  "content": "<图片中的关键文字摘要>"
}
`, officialName, officialName)
}

// imageToBase64 (新的辅助函数，返回 base64 和 mimeType)
func imageToBase64(fileHeader *multipart.FileHeader) (string, string, error) {
	// 1. 获取 MIME 类型
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		return "", "", fmt.Errorf("不支持的图片格式: %s，仅支持 jpeg 或 png", mimeType)
	}

	// 2. 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	// 3. 读取文件所有字节
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", "", err
	}

	// 4. 编码为 Base64
	sEnc := base64.StdEncoding.EncodeToString(fileBytes)
	return sEnc, mimeType, nil
}
