package client

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"time"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"log"
	"mime/multipart"

	"github.com/nfnt/resize"
)

// VisionMessage 多模态消息
type VisionMessage struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"` // 内容是一个列表
}

// ContentPart 消息的组成部分 (text 或 image_url)
type ContentPart struct {
	Type     string               `json:"type"`                // "text" 或 "image_url"
	Text     string               `json:"text,omitempty"`      // type="text" 时使用
	ImageURL *ChatMessageImageURL `json:"image_url,omitempty"` // type="image_url" 时使用
}

// ChatMessageImageURL 图片 URL (data:...)
type ChatMessageImageURL struct {
	URL string `json:"url"` // "data:image/jpeg;base64,..."
}

// LlmResponse (通用的响应结构)
type LlmResponse struct {
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

func imageToBase64(fileHeader *multipart.FileHeader) (string, string, error) {

	file, err := fileHeader.Open()
	if err != nil {
		return "", "", fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	img, originalFormat, err := image.Decode(file)
	if err != nil {
		return "", "", fmt.Errorf("无法解码图片: %w", err)
	}
	log.Printf("图片原始格式: %s, 原始尺寸: %dx%d", originalFormat, img.Bounds().Dx(), img.Bounds().Dy())

	// 缩放图片以控制文件大小
	const maxWidth uint = 1000
	const maxHeight uint = 1000

	// 如果图片太大，进行缩放
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

func getBase64FromInput(imageURL string) (string, string, error) {
	if imageURL == "" {
		return "", "", fmt.Errorf("没有提供图片 URL")
	}

	log.Printf("Processing image from URL: %s", imageURL)

	httpClient := &http.Client{Timeout: 10 * time.Second} // 10秒下载超时
	resp, err := httpClient.Get(imageURL)
	if err != nil {
		return "", "", fmt.Errorf("无法下载图片 URL: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return "", "", fmt.Errorf("下载图片 URL 失败, 状态码: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	return processImageStream(resp.Body)
}

// processImageInput 统一处理图片输入（支持 fileHeader 或 imageURL）
// 返回: base64编码的图片, MIME类型, 错误
func processImageInput(fileHeader *multipart.FileHeader, imageURL string) (string, string, error) {
	// 优先使用 fileHeader，其次使用 imageURL
	if fileHeader != nil {
		log.Printf("处理上传的文件: %s (大小: %d bytes)", fileHeader.Filename, fileHeader.Size)
		return imageToBase64(fileHeader)
	}

	if imageURL != "" {
		log.Printf("处理图片 URL: %s", imageURL)
		return getBase64FromInput(imageURL)
	}

	return "", "", fmt.Errorf("没有提供图片文件或图片 URL")
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

func buildExtractorPrompt(officialName string, appType string) string {

	// 帮助 AI 理解上下文
	var appTypeContext string
	if appType == "病假" {
		appTypeContext = "请病假（例如：病历单、处方单、诊断证明）"
	} else if appType == "补打卡" {
		appTypeContext = "补打卡（例如：系统截图、浏览器记录、能证明在办公区的时间记录）"
	} else {
		appTypeContext = appType + "（例如：能证明当前请假类型的有效单据、图片、截图等）"
	}

	return fmt.Sprintf(`
你是一个专业且严谨的人力资源（HR）助理。
你的任务是分析这张图片，它被作为一份证据提交。

**上下文信息**:
1.  **目标员工姓名**: %s
2.  **员工申请类型**: %s

**你的任务 (请严格按步骤执行)**:
1.  **验证姓名 (!! 关键甄别 !!)**: 在图片中查找 **看诊人（患者）** 的姓名。**请注意甄别**，不要错误地提取医生的姓名。如果找到，提取为 "extracted_name"。
2.  **提取日期**: 提取图片中的日期 (request_date)，格式 "yyyy-MM-dd"。
3.  **提取时间**: 提取图片中的时间 (request_time)，格式 "HH:mm"。
4.  **识别类型**: 识别这张图片的类型 (request_type)，例如："病历单", "浏览器记录", "未知"。
5.  **判断有效性 (!! 关键 !!)**: 根据“员工申请类型是「%s」”这个上下文，判断这张图片是否为**合理且有效**的证明材料。
    * 例如：如果申请 "病假"，图片是 "病历单" -> true。
    * 例如：如果申请 "病假"，图片是 "浏览器记录" -> false。
    * 例如：如果申请 "补打卡"，图片是 "浏览器记录" -> true。
    * 例如：如果申请 "补打卡"，图片是 "病历单" -> false。
6.  **提取内容**: 提取图片中的关键文字 (content)，例如病历单的诊断结果。
7.  如果信息不存在，将对应字段的值设为 "未知"。

**JSON格式 (必须严格返回此格式)**:
{
  "extracted_name": "<图片上找到的姓名>",
  "request_date": "<图片上的日期>",
  "request_time": "<图片上的时间>",
  "request_type": "<判断出的图片类型>",
  "is_proof_type_valid": <true_or_false>, 
  "content": "<图片中的关键文字摘要>"
}
`, officialName, appType, appTypeContext)
}
