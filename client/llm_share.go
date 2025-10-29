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

	_ "image/gif"
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
func buildExtractorPrompt1(appName string, appType string, appDate string, appStart string, appEnd string) string {
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

// vision模型下准确率高，但是速度慢
func buildExtractorPrompt(appName string, appType string, appDate string, appStart string, appEnd string) string {
	// 规范化申请时间显示
	appTime := displayAppTime(appStart, appEnd)
	// 当前未接入外部OA，考勤数据占位
	attendanceData := "N/A"

	// 构建最终 Prompt（严格按用户提供的输入/输出规范）
	return fmt.Sprintf(`
你是一位专业的图像和信息分析专家，擅长处理考勤申请相关的分析工作。你的任务是根据提供的考勤申请图片以及相关员工考勤信息，对申请的有效性进行分析判断，并严格按指定JSON输出。

输入：
- 考勤申请图片：由系统在同一消息的 image_url 部分提供
- 申请的类型：%s
- 员工姓名：%s（若为空则不需校验）
- 申请的日期：%s
- 申请的时间：%s
- 员工当天的考勤数据：%s

分析判断要求：
1. 若传入了员工姓名，需判断提取的姓名是否和申请人一致。
2. 判断图片中提取到的最相关日期是否与 {{APPLICATION_DATE}} 一致。
3. 判断图片时间是否有符合申请的时间点。
4. 原则: 基于 {{APPLICATION_TYPE}}，判断该图片证据是否能从逻辑上强有力地支撑申请事由。分析指引:
若为 "病假": 证据是否能证明申请人(员工)在申请日期确实因医疗原因无法工作？（例如：诊断证明、挂号单、药费单等）。
若为 "补打卡" (含上下班): 证据是否能合理且可信地证明员工在工作或者在公司内？
判断标准: AI应自主评估证据的可信度。无需局限于特定类型。
有效证据示例:
物理在司证明: 饭堂/内部消费小票, 门禁刷卡记录, 包含公司环境的带时间戳照片, 办公楼下快递签收记录等。
数字在司证明: 电脑系统日志 (如 事件查看器 (Event Viewer), 开关机记录), 内部OA/ERP/Git/Jira等系统操作截图, VPN登录记录, 有上下文的(显示了工作内容)且带时间戳的工作聊天记录。
AI 指引: AI应优先采信这些类别的证据，只要它们能清晰展示时间戳并与员工的工作相关联（如系统日志能证明电脑在运行），就应视为有效（true），而不是因其无法同时满足所有绑定条件（如“事件查看器”无法直接绑定“员工”）而拒绝。
若为 "其他": 证据是否能支持申请人提出的具体事由？
5. 提取图片中的关键字摘要（≤60字，不要重复时间）。
6. 判断图片是否为聊天记录。
7. 分析并给出符合 / 不符合的原因，需综合考虑图片内容、时间、考勤数据等多方面因素导致申请无效的情况。
8. 给出AI的建议，即是否建议通过该申请。

输出格式要求：严格输出以下JSON（不要多余解释）：
{
  "name_match": true/false,
  "date_match": true/false,
  "time_match": true/false,
  "type_match": true/false,
  "keywords": "",
  "is_chat_record": true/false,
  "reason": "",
  "approve": true/false
}
`, appType, appName, appDate, appTime, attendanceData)
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

// 针对不需要图片校验的场景，基于申请参数与考勤信息进行规则性评估的 Prompt
func buildCheckByNoImagePrompt(appType string, appName string, appDate string, appStart string, appEnd string, attendanceInfo []string) string {
	appTime := displayAppTime(appStart, appEnd)
	var attendanceText string
	if len(attendanceInfo) == 0 {
		attendanceText = "无"
	} else {
		attendanceText = fmt.Sprintf("%v", attendanceInfo)
	}

	return fmt.Sprintf(`
你是HR考勤与审批助手。在无需图片核验的前提下，仅依据申请参数与当日考勤数据，判断当日属性并评估申请合理性。严格返回指定JSON，不要多余解释。

输入：
- 申请类型：%s
- 员工姓名：%s（若为空可忽略姓名一致性）
- 申请日期：%s
- 申请时间：%s
- 当日考勤时间点（HH:mm 列表，上下班/打卡记录）：%s

要求：
1) 先判断该日期是工作日还是节假日：
   - 如无法联网获取法定节假日，按周一~周五视为“工作日”，周六/周日视为“节假日”，并在 reason 中注明依据。
2) 基于当日属性与“申请类型”评估“是否合理”：
   - 请假类（事假/病假/年假/调休等）→ 工作日更合理；
   - 节假日加班/加班调休 → 节假日更合理；
   - 补打卡 → 根据考勤时间点是否与申请时间段存在合理对应；
3) 若存在当日考勤数据：
   - 如已存在完整上下班记录但申请“请假”（全天）→ 判为“矛盾”；
   - 如无任何打卡但申请“补打卡”（仅特定时间点）→ 判定是否存在可解释的合理性；
   - 无考勤数据则标记为“无数据”。

输出：严格返回以下JSON（不要包裹markdown或注释）：
{
  "is_work_day": true/false,
  "day_type": "工作日/节假日",
  "application_reasonable": true/false,
  "attendance_consistency": "一致/矛盾/无数据",
  "reason": "",
  "suggestion": ""
}
`, appType, appName, appDate, appTime, attendanceText)
}