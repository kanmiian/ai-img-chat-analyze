package service

import (
	"fmt"
	"log"
	"my-ai-app/model"
	"strings"
	"time"
)

// TimeValidator 时间验证器
type TimeValidator struct{}

// NewTimeValidator 创建时间验证器
func NewTimeValidator() *TimeValidator { return &TimeValidator{} }

// ValidateApplicationTime 验证申请时间的有效性
func (tv *TimeValidator) ValidateApplicationTime(appData model.ApplicationData) (*model.TimeValidationResult, error) {
	log.Printf("开始验证申请时间 - UserId: %s, Date: %s, Time: %s, Type: %s",
		appData.UserId, appData.ApplicationDate, appData.ApplicationTime, appData.ApplicationType)

	// 不再调用 OA 接口，直接基于申请数据进行基础校验
	result := tv.createBasicValidationResult(appData)

	log.Printf("时间验证完成 - IsValid: %v, IsWorkDay: %v, IsLate: %v, RiskLevel: %s",
		result.IsValid, result.IsWorkDay, result.IsLate, result.RiskLevel)

	return result, nil
}

// performTimeValidation 执行具体的时间验证逻辑
// 已不再依赖 OA 返回的字段，保留 isLate 等工具函数用于基础判断

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
