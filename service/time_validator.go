package service

import (
	"fmt"
	"log"
	"my-ai-app/client"
	"my-ai-app/model"
	"strings"
	"time"
)

// TimeValidator 时间验证器
type TimeValidator struct {
	oaClient *client.OaClient
}

// NewTimeValidator 创建时间验证器
func NewTimeValidator(oaClient *client.OaClient) *TimeValidator {
	return &TimeValidator{
		oaClient: oaClient,
	}
}

// ValidateApplicationTime 验证申请时间的有效性
func (tv *TimeValidator) ValidateApplicationTime(appData model.ApplicationData) (*model.TimeValidationResult, error) {
	log.Printf("开始验证申请时间 - UserId: %s, Date: %s, Time: %s, Type: %s",
		appData.UserId, appData.ApplicationDate, appData.ApplicationTime, appData.ApplicationType)

	// 1. 从OA系统获取考勤数据
	attendanceData, err := tv.oaClient.GetAttendanceData(appData.UserId, appData.ApplicationDate)
	if err != nil {
		log.Printf("获取考勤数据失败: %v", err)
		// 如果无法获取考勤数据，返回基础验证结果
		return tv.createBasicValidationResult(appData), nil
	}

	// 2. 执行时间验证逻辑
	result := tv.performTimeValidation(appData, attendanceData)

	log.Printf("时间验证完成 - IsValid: %v, IsWorkDay: %v, IsLate: %v, RiskLevel: %s",
		result.IsValid, result.IsWorkDay, result.IsLate, result.RiskLevel)

	return result, nil
}

// performTimeValidation 执行具体的时间验证逻辑
func (tv *TimeValidator) performTimeValidation(appData model.ApplicationData, attendanceData *client.AttendanceData) *model.TimeValidationResult {
	result := &model.TimeValidationResult{
		IsValid:   true,
		IsWorkDay: attendanceData.IsWorkDay,
		IsLate:    false,
		RiskLevel: "low",
	}

	// 检查是否为工作日
	if !attendanceData.IsWorkDay {
		result.IsValid = false
		result.RiskLevel = "high"
		result.Suggestion = "申请时间为非工作日，请确认申请类型"
		result.Details = fmt.Sprintf("申请日期 %s 不是工作日", appData.ApplicationDate)
		return result
	}

	// 检查申请类型
	if appData.ApplicationType == "补打卡" {
		// 补打卡申请需要检查是否会导致迟到
		if tv.isLate(appData.ApplicationTime, attendanceData.WorkStartTime) {
			result.IsLate = true
			result.RiskLevel = "medium"
			result.Suggestion = "申请时间可能导致迟到，请确认是否合理"
			result.Details = fmt.Sprintf("申请时间 %s 晚于标准上班时间 %s",
				appData.ApplicationTime, attendanceData.WorkStartTime)
		} else {
			result.Suggestion = "申请时间合理，符合补打卡要求"
			result.Details = fmt.Sprintf("申请时间 %s 早于标准上班时间 %s",
				appData.ApplicationTime, attendanceData.WorkStartTime)
		}
	} else if appData.ApplicationType == "病假" {
		// 病假申请通常不需要时间验证
		result.Suggestion = "病假申请，时间验证通过"
		result.Details = "病假申请无需验证具体时间"
	} else {
		// 其他类型的申请
		result.Suggestion = "申请时间验证通过"
		result.Details = "申请时间符合要求"
	}

	return result
}

// isLate 判断申请时间是否会导致迟到
func (tv *TimeValidator) isLate(applicationTime, standardTime string) bool {
	// 解析时间
	appTime, err1 := time.Parse("15:04", applicationTime)
	standardTimeParsed, err2 := time.Parse("15:04", standardTime)

	if err1 != nil || err2 != nil {
		log.Printf("时间解析失败 - ApplicationTime: %s, StandardTime: %s", applicationTime, standardTime)
		return false
	}

	return appTime.After(standardTimeParsed)
}

// createBasicValidationResult 创建基础验证结果（当无法获取考勤数据时）
func (tv *TimeValidator) createBasicValidationResult(appData model.ApplicationData) *model.TimeValidationResult {
	// 基础的工作日判断
	parsedDate, err := time.Parse("2006-01-02", appData.ApplicationDate)
	isWorkDay := true
	if err == nil {
		weekday := parsedDate.Weekday()
		isWorkDay = weekday >= time.Monday && weekday <= time.Friday
	}

	result := &model.TimeValidationResult{
		IsValid:   true,
		IsWorkDay: isWorkDay,
		IsLate:    false,
		RiskLevel: "medium",
	}

	if !isWorkDay {
		result.IsValid = false
		result.RiskLevel = "high"
		result.Suggestion = "申请时间为非工作日，请确认申请类型"
		result.Details = fmt.Sprintf("申请日期 %s 不是工作日", appData.ApplicationDate)
	} else {
		result.Suggestion = "无法获取考勤数据，请人工审核"
		result.Details = "系统无法验证考勤情况，建议人工审核"
	}

	return result
}

// GenerateValidationMessage 生成验证消息
func (tv *TimeValidator) GenerateValidationMessage(result *model.TimeValidationResult) string {
	var messages []string

	// 基础信息
	if result.IsWorkDay {
		messages = append(messages, "申请时间为工作日")
	} else {
		messages = append(messages, "申请时间为非工作日")
	}

	// 迟到信息
	if result.IsLate {
		messages = append(messages, "申请时间可能导致迟到")
	}

	// 风险级别
	switch result.RiskLevel {
	case "high":
		messages = append(messages, "高风险申请")
	case "medium":
		messages = append(messages, "中等风险申请")
	case "low":
		messages = append(messages, "低风险申请")
	}

	// 建议
	if result.Suggestion != "" {
		messages = append(messages, result.Suggestion)
	}

	return strings.Join(messages, "；")
}
