package client

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

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
	startTime := time.Now()
	log.Printf("开始处理图片 - 文件名: %s, 大小: %d bytes", fileHeader.Filename, fileHeader.Size)

	// 1. 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("打开文件失败: %v", err)
		return "", "", err
	}
	defer file.Close()

	// 2. 读取文件内容到内存
	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("读取文件内容失败: %v", err)
		return "", "", fmt.Errorf("读取文件内容失败: %w", err)
	}
	log.Printf("文件读取完成 - 大小: %d bytes", len(fileData))

	// 3. 从字节数据解码图片
	img, originalFormat, err := image.Decode(bytes.NewReader(fileData))
	if err != nil {
		log.Printf("图片解码失败 - 格式: %s, 错误: %v", originalFormat, err)
		// 尝试直接使用原始数据作为Base64
		log.Printf("尝试直接使用原始数据作为Base64")
		base64Data := base64.StdEncoding.EncodeToString(fileData)
		// 根据文件扩展名确定MIME类型
		mimeType := "image/jpeg" // 默认
		if fileHeader.Filename != "" {
			ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
			switch ext {
			case ".png":
				mimeType = "image/png"
			case ".jpg", ".jpeg":
				mimeType = "image/jpeg"
			case ".gif":
				mimeType = "image/gif"
			case ".webp":
				mimeType = "image/webp"
			}
		}
		log.Printf("使用原始数据Base64 - MIME类型: %s", mimeType)
		return base64Data, mimeType, nil
	}
	log.Printf("图片解码成功 - 格式: %s, 尺寸: %dx%d", originalFormat, img.Bounds().Dx(), img.Bounds().Dy())

	// 4. 缩放图片
	const maxWidth uint = 1200
	if img.Bounds().Dx() > int(maxWidth) {
		img = resize.Resize(maxWidth, 0, img, resize.Lanczos3)
		log.Printf("图片已缩放至宽度: %dpx", maxWidth)
	}

	// 5. 将图片重编码为 JPEG
	encodeStartTime := time.Now()
	buf := new(bytes.Buffer)
	jpegOptions := &jpeg.Options{Quality: 80}
	if err := jpeg.Encode(buf, img, jpegOptions); err != nil {
		log.Printf("JPEG编码失败: %v", err)
		return "", "", fmt.Errorf("无法将图片编码为 JPEG: %w", err)
	}
	encodeDuration := time.Since(encodeStartTime)
	log.Printf("JPEG编码完成 (耗时: %v, 大小: %d bytes)", encodeDuration, buf.Len())

	// 6. 编码为 Base64
	base64StartTime := time.Now()
	sEnc := base64.StdEncoding.EncodeToString(buf.Bytes())
	base64Duration := time.Since(base64StartTime)

	totalDuration := time.Since(startTime)
	log.Printf("图片处理完成 - 总耗时: %v (编码: %v, Base64: %v), 最终大小: %d chars",
		totalDuration, encodeDuration, base64Duration, len(sEnc))

	// 7. 返回 Base64 和 JPEG 的 MIME 类型
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
		appTypeContext = appType
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
`, officialName, appType, appTypeContext) // <-- 注入所有上下文
}
