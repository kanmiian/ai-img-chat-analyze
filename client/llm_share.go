// client/llm_share.go
package client

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"

	// (解码器保持不变)
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	// 移除对 golang.org/x/image 的依赖
	// 如果后续需要支持 BMP 和 WebP，可以使用 github.com/disintegration/imaging 替代
	// _ "golang.org/x/image/bmp"
	// _ "golang.org/x/image/webp"

	// (确保导入)

	"github.com/nfnt/resize"
)

// ... (VisionMessage, ContentPart, ChatMessageImageURL, LlmResponse 结构体保持不变) ...
type VisionMessage struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}
type ContentPart struct {
	Type     string               `json:"type"`
	Text     string               `json:"text,omitempty"`
	ImageURL *ChatMessageImageURL `json:"image_url,omitempty"`
}
type ChatMessageImageURL struct {
	URL string `json:"url"`
}
type LlmResponse struct {
	Id      string      `json:"id"`    // LLM API返回的请求ID
	Usage   *TokenUsage `json:"usage"` // Token使用情况
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

// TokenUsage Token使用情况
type TokenUsage struct {
	CompletionTokens int `json:"completion_tokens"` // 生成的token数
	PromptTokens     int `json:"prompt_tokens"`     // 输入的token数
	TotalTokens      int `json:"total_tokens"`      // 总token数
}

func processImageStream(imageStream io.Reader) (string, string, error) {
	img, originalFormat, err := image.Decode(imageStream)
	if err != nil {
		return "", "", fmt.Errorf("无法解码图片: %w", err)
	}
	log.Printf("图片原始格式: %s, 原始尺寸: %dx%d", originalFormat, img.Bounds().Dx(), img.Bounds().Dy())
	const maxWidth uint = 1000
	const maxHeight uint = 1000
	if img.Bounds().Dx() > int(maxWidth) || img.Bounds().Dy() > int(maxHeight) {
		img = resize.Resize(maxWidth, maxHeight, img, resize.Lanczos3)
		log.Printf("图片已缩放至: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	buf := new(bytes.Buffer)
	jpegOptions := &jpeg.Options{Quality: 80}
	if err := jpeg.Encode(buf, img, jpegOptions); err != nil {
		return "", "", fmt.Errorf("无法将图片编码为 JPEG: %w", err)
	}
	log.Printf("图片已重编码为 JPEG (质量: 80), 压缩后大小: %.2f KB", float64(buf.Len())/1024.0)
	sEnc := base64.StdEncoding.EncodeToString(buf.Bytes())
	return sEnc, "image/jpeg", nil
}

// buildImageContentPart 构建图片内容部分
func buildImageContentPart(fileHeader *multipart.FileHeader, imageURL string) (*ContentPart, error) {
	// 处理文件上传
	if fileHeader != nil {
		log.Printf("使用base64方式处理文件: %s (大小: %d bytes)", fileHeader.Filename, fileHeader.Size)
		// 打开文件
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("无法打开文件: %w", err)
		}
		defer file.Close()

		// 处理图片流
		base64Image, mimeType, err := processImageStream(file)
		if err != nil {
			return nil, fmt.Errorf("base64编码失败: %w", err)
		}

		// 构建data URI
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)
		return &ContentPart{
			Type:     "image_url",
			ImageURL: &ChatMessageImageURL{URL: dataURI},
		}, nil
	}

	// 处理URL
	if imageURL != "" {
		return &ContentPart{
			Type:     "image_url",
			ImageURL: &ChatMessageImageURL{URL: imageURL},
		}, nil
	}

	return nil, fmt.Errorf("没有提供图片文件或图片URL")
}

// (!! ---------------- 关键修改：简化的 Prompt ---------------- !!)
func buildExtractorPrompt(appName string, appType string, appDate string, appStart string, appEnd string) string {
	var appTypeContext string
	switch appType {
	case "病假":
		appTypeContext = "病历单、处方单、诊断证明"
	case "补打卡":
		appTypeContext = "饭堂消费/系统操作/电脑开关机记录、聊天记录、含时间的办公环境照片等"
	default:
		appTypeContext = fmt.Sprintf("能证明%s的有效单据/图片/截图（含时间的办公环境照片等优先）", appType)
	}

	// 根据申请类型动态调整姓名提取提示（病假→患者，其他→员工）
	var nameExtractHint string
	switch appType {
	case "病假":
		nameExtractHint = "患者/看诊人"
	default:
		nameExtractHint = "当事人/员工姓名"
	}

	var appNameContext string
	if appName != "" {
		appNameContext = fmt.Sprintf("1. 目标员工姓名：%s（优先提取图片中%s，排除无关人员如医生）", appName, nameExtractHint)
	} else {
		appNameContext = fmt.Sprintf("1. 目标员工姓名：未提供，请提取图片中%s姓名（排除无关人员如医生）", nameExtractHint)
	}

	appTime := displayAppTime(appStart, appEnd)

	var punchRule string
	if appType == "补打卡" {
		punchRule = `**补打卡证据要求**：
- 有效性条件（需同时满足）：
  1. 证据类型属于以下之一：含时间的办公环境照片、聊天记录（含时间）、饭堂消费记录、系统操作记录、电脑开关机记录；
  2. 证据中的日期（request_date，即图片日期）≤申请日期（上下文3中的申请日期）；
- 时间提取规则：
  1. 若申请为上班卡，优先选≤申请时间的最近时间；若为下班卡，优先选≥申请时间的最近时间（办公环境照片的显示时间直接认定为有效时间）；
  2. 候选时间最多返回5个`
	}

	year := "（仅月日补此年份）"
	if len(appDate) >= 4 {
		year = appDate[:4] + year
	}

	return fmt.Sprintf(`
你是HR助理，需从证据图提取信息并严格返回JSON。

**上下文**：
%s
2. 申请类型：%s（有效证据：%s）
3. 申请日期：%s
4. 申请时间：%s

%s

**任务**：提取JSON（严格按格式）：
- extracted_name：图片中%s姓名（排除无关人员如医生）
- request_date：图片日期（yyyy-MM-dd，仅月日补%s）
- request_time：图片时间（HH:mm，优先符合申请时间）
- request_type：图片类型（病历单/聊天记录/含时间的办公环境照片/饭堂消费记录/系统操作记录/电脑开关机记录/未知）
- is_proof_type_valid：是否为%s有效证据（补打卡需满足上述补打卡证据要求中的有效性条件；病假需为病历单、处方单或诊断证明；其他类型需为含时间的办公环境照片）
- content：关键文字摘要（≤60字，不重复时间）
- is_company_internal：是否为公司内部场景（如工位、饭堂、办公系统）
- is_chat_record：是否为聊天记录
- time_from_content：从聊天记录内容中提取的时间（HH:mm，无则空）
- candidate_times：候选时间数组（当图片为聊天记录/含时间的办公环境照片/饭堂消费记录/系统操作记录/电脑开关机记录时，提取内容中所有时间点，最多5个）

**JSON格式**：
{"extracted_name":"","request_date":"","request_time":"","request_type":"","is_proof_type_valid":true/false,"content":"","is_company_internal":true/false,"is_chat_record":false,"time_from_content":"","candidate_times":[]}
`, appNameContext, appType, appTypeContext, appDate, appTime, punchRule, nameExtractHint, year, appType)
}

func displayAppTime(appStart, appEnd string) string {
	switch {
	case appStart != "" && appEnd != "":
		return fmt.Sprintf("%s~%s", appStart, appEnd)
	case appStart != "":
		return appStart
	case appEnd != "":
		return appEnd
	default:
		return "未提供"
	}
}
