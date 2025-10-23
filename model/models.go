package model

// ApplicationData OA 系统提交的表单数据
type ApplicationData struct {
	UserId          string `form:"user_id"`          // 员工 ID
	Alias           string `form:"alias"`            // 员工姓名
	ApplicationType string `form:"application_type"` // 申请类型 (e.g., "补打卡", "病假")
	ApplicationTime string `form:"application_time"` // 申请的时间 (e.g., "09:00")
	ApplicationDate string `form:"application_date"` // 申请的日期 (e.g., "2025-10-21")
	Reason          string `form:"reason"`           // 申请理由 (文字)
}

// ExtractedData 是从(图片)中提取的结构化数据
// (这个结构体可以保持不变，或者根据需要扩展)
type ExtractedData struct {
	ExtractedName    string `json:"extracted_name"`
	RequestDate      string `json:"request_date"`
	RequestTime      string `json:"request_time"`
	RequestType      string `json:"request_type"`
	IsProofTypeValid bool   `json:"is_proof_type_valid"`
	Content          string `json:"content"`
}

// AnalysisResult 是我们 API 统一的返回结构 (保持不变)
type AnalysisResult struct {
	IsAbnormal bool   `json:"is_abnormal"`
	Reason     string `json:"reason"`
	// ExtractedData ExtractedData `json:"data,omitempty"` // 把从图片提取的数据也返回
	RawText string `json:"raw_text,omitempty"` // 调试文本
}
