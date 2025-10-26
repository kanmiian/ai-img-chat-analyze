package model

// ApplicationData OA 系统提交的表单数据
type ApplicationData struct {
	UserId          string   `form:"user_id"`              // 员工 ID
	Alias           string   `form:"alias"`                // 员工姓名
	ApplicationType string   `form:"application_type"`     // 申请类型 (e.g., "补打卡", "病假")
	ApplicationTime string   `form:"application_time"`     // 申请的时间 (e.g., "09:00")
	ApplicationDate string   `form:"application_date"`     // 申请的日期 (e.g., "2025-10-21")
	Reason          string   `form:"reason"`               // 申请理由 (文字)
	ImageUrl        string   `form:"image_url"`            // 图片 URL（单个，向后兼容）
	ImageUrls       []string `form:"image_urls[]"`         // 图片 URLs（多个）
	UseDirectUrl    string   `form:"use_direct_url"`       // 是否直接使用图片 URL，不进行 Base64 转换（true/false）
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

// AttendanceData OA系统返回的考勤数据
type AttendanceData struct {
	UserId         string `json:"user_id"`         // 员工ID
	WorkDate       string `json:"work_date"`       // 工作日期 YYYY-MM-DD
	WorkStartTime  string `json:"work_start_time"` // 上班时间 HH:mm
	WorkEndTime    string `json:"work_end_time"`   // 下班时间 HH:mm
	IsWorkDay      bool   `json:"is_work_day"`     // 是否工作日
	AttendanceType string `json:"attendance_type"` // 考勤类型: normal, late, absent
}

// TimeValidationResult 时间验证结果
type TimeValidationResult struct {
	IsValid    bool   `json:"is_valid"`    // 是否有效
	IsWorkDay  bool   `json:"is_work_day"` // 是否工作日
	IsLate     bool   `json:"is_late"`     // 是否迟到
	RiskLevel  string `json:"risk_level"`  // 风险级别: low, medium, high
	Suggestion string `json:"suggestion"`  // 建议
	Details    string `json:"details"`     // 详细信息
}

// ImageAnalysisDetail 单张图片的分析详情
type ImageAnalysisDetail struct {
	Index            int            `json:"index"`                    // 图片索引（从1开始）
	Source           string         `json:"source"`                   // 来源：file_upload 或 url_download
	FileName         string         `json:"file_name,omitempty"`      // 文件名（文件上传时）
	ImageURL         string         `json:"image_url,omitempty"`      // 图片URL（URL下载时）
	Success          bool           `json:"success"`                  // 是否分析成功
	ErrorMessage     string         `json:"error_message,omitempty"`  // 错误信息
	ExtractedData    *ExtractedData `json:"extracted_data,omitempty"` // 提取的数据
	ProcessingTimeMs int64          `json:"processing_time_ms"`       // 处理时间（毫秒）
	IsValid          bool           `json:"is_valid"`                 // 是否为有效证明材料
}

// AnalysisResult 是我们 API 统一的返回结构
type AnalysisResult struct {
	IsAbnormal      bool                  `json:"is_abnormal"`
	Reason          string                `json:"reason"`
	ValidImageIndex int                   `json:"valid_image_index,omitempty"` // 有效图片的索引（从1开始，0表示无）
	ImagesAnalysis  []ImageAnalysisDetail `json:"images_analysis,omitempty"`   // 所有图片的分析详情
	TimeValidation  *TimeValidationResult `json:"time_validation,omitempty"`   // 时间验证结果（已废弃）
	RawText         string                `json:"raw_text,omitempty"`          // 调试文本
}
