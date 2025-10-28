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

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

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
		log.Printf("直接传递URL（跳过下载和编码）: %s", imageURL)
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
	if appType == "病假" {
		appTypeContext = "请病假（例如：病历单、处方单、诊断证明）"
	} else if appType == "补打卡" {
		appTypeContext = "补打卡（关键是证明工作时间，例如：饭堂/食堂消费记录、系统/网页操作记录、电脑开关机时间、聊天记录、系统截图、或任何显示日期和时间的电脑桌面截图）"
	} else {
		appTypeContext = appType + "（例如：能证明当前请假类型的有效单据、图片、截图等）"
	}

	var appNameContext string
	if appName != "" {
		appNameContext = fmt.Sprintf("1.  **目标员工姓名**: %s (请在图片中查找此人，并将其作为'看诊人'或'患者'来识别)", appName)
	} else {
		appNameContext = "1.  **目标员工姓名**: (未提供), 请提取图片中的'看诊人'或'患者'的姓名"
	}

	// (!! ---------------- 简化的 Prompt ---------------- !!)
	return fmt.Sprintf(`
你是一个严谨的 HR 助理，负责分析证据图片。

**上下文**:
%s
2.  **员工申请类型**: %s (有效证据包括: %s)
3.  **员工申请日期**: %s
4.  **员工申请时间**: %s

**任务**:
严格按照 JSON 格式，从图片中提取以下信息：
1.  **extracted_name**: 图片中医患相关的姓名（优先患者，甄别医生）。
2.  **request_date**: 图片中的日期 (yyyy-MM-dd)。如果只有月日，请结合申请日期 (%s) 推断年份。
3.  **request_time**: 图片中的时间 (HH:mm)。**重要**：如果图片有多个时间，请选择最符合申请时间要求的时间：
    - 上班卡申请：选择≤申请时间且最接近的时间
    - 下班卡申请：选择≥申请时间且最接近的时间
    - 区间申请：选择在申请时间区间内的时间，若无则选择最接近的时间
3b. **candidate_times**: (数组) 若为聊天记录或出现多条时间，请根据申请时间智能筛选：
    - 上班卡申请：返回≤申请时间且最接近的时间
    - 下班卡申请：返回≥申请时间且最接近的时间  
    - 区间申请：返回在申请时间区间内的时间，若无则返回最接近的时间
    - 例如：申请13:30上班卡，图片有[17:01, 17:02, 17:34, 17:40]，应返回[]（都晚于13:30）
    - 例如：申请18:00下班卡，图片有[17:01, 17:02, 17:34, 17:40]，应返回["17:40"]（最接近且≥18:00）
4.  **request_type**: 图片的类型 (例如: "病历单", "聊天记录", "桌面截图", "未知")。
5.  **is_proof_type_valid**: (布尔值) 这张图片能否作为"%s"申请的**有效证据**？
    * **!! 补打卡特别规则 !!**: 如果申请类型是 "补打卡"，以下类型**必须**视为有效证据（返回 true）：
      - **饭堂/食堂消费记录**：任何包含"饭堂/食堂/消费/就餐/餐饮/餐费/餐卡"字样的图片，包括"账单详情"、"消费记录"、"支付记录"等
      - **系统操作记录**：电脑开关机、网页浏览、软件操作等带明确日期时间的截图
      - **聊天记录**：虽然时间可能有效，但建议提供更直接的办公证明
    * **重要**：只要图片能清晰体现日期与时间，且属于上述类型之一，**必须返回 true**，不要因为"不在办公区"而拒绝。
    * **病假规则**: 如果申请 "病假"，"病历单" 或 "诊断证明" 是 **true**。
    * **其他**: 如果类型不匹配 (如用 "病历单" 补打卡)，则为 **false**。
6.  **content**: 图片中的关键文字摘要 (如诊断结果或聊天内容)。**重要**：内容摘要不超过60字，避免重复列举时间。
7.  **is_company_internal**: (布尔值) 图片是否显示为公司内部（如工位、办公环境等）？。
8.  **is_chat_record**: (布尔值) 图片是否为聊天记录截图？
9.  **time_from_content**: (HH:mm) 如果是聊天记录或截图，从内容中提取的时间。
10. **candidate_times**: (数组) 若为聊天记录或出现多条时间，请根据申请时间智能筛选：
    - 上班卡申请：返回≤申请时间且最接近的时间
    - 下班卡申请：返回≥申请时间且最接近的时间  
    - 区间申请：返回在申请时间区间内的时间，若无则返回最接近的时间
    - **重要**：最多返回5个时间，按相关性排序，避免输出过长
    - 例如：申请13:30上班卡，图片有[17:01, 17:02, 17:34, 17:40]，应返回[]（都晚于13:30）
    - 例如：申请18:00下班卡，图片有[17:01, 17:02, 17:34, 17:40]，应返回["17:40"]（最接近且≥18:00）

**!! 补打卡时间验证重要说明 !!**:
- 对于补打卡申请，证明材料中的时间必须**早于或等于**申请时间才能有效
- 例如：申请9:05补打卡，证明材料时间18:22是无效的（无法证明9:05前在公司）
- 例如：申请9:05补打卡，证明材料时间8:30是有效的（可以证明9:05前在公司）

**JSON格式 (必须严格返回)**:
{
  "extracted_name": "...",
  "request_date": "...",
  "request_time": "...",
  "request_type": "...",
  "is_proof_type_valid": <true_or_false>, 
  "content": "...",
  "is_company_internal": <true_or_false>,
  "is_chat_record": <true_or_false>,
  "time_from_content": "...",
  "candidate_times": ["08:59", "09:02", "18:32"]
}
`, appNameContext, appType, appTypeContext, appDate, displayAppTime(appStart, appEnd), appDate, appType)
}

func displayAppTime(appStart, appEnd string) string {
	if appStart != "" && appEnd != "" {
		return fmt.Sprintf("%s ~ %s", appStart, appEnd)
	}
	if appStart != "" {
		return appStart
	}
	if appEnd != "" {
		return appEnd
	}
	return "(未提供)"
}
