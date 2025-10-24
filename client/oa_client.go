package client

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// OaClient 负责与 OA 系统的 API 通信
type OaClient struct {
	baseURL    string
	httpClient *http.Client
}

// EmployeeData (OA系统返回的员工基准数据)
type EmployeeData struct {
	UserId string `json:"user_id"`
	Alias  string `json:"alias"`
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

func NewOaClient(baseURL string) *OaClient {
	return &OaClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetEmployeeData 从 OA 系统获取员工的基准数据 todo 调整员工信息的判断
func (c *OaClient) GetEmployeeData(employeeID string) (*EmployeeData, error) {
	// 临时返回模拟数据（开发环境）
	return &EmployeeData{
		UserId: employeeID,
		Alias:  "测试员工",
	}, nil
}

// GetAttendanceData 从 OA 系统获取员工的考勤数据
func (c *OaClient) GetAttendanceData(employeeID string, workDate string) (*AttendanceData, error) {
	log.Printf("开始获取员工考勤数据 - EmployeeID: %s, WorkDate: %s", employeeID, workDate)
	// todo 调取接口？ 或者是在请求的时候一起带过来
	// 临时返回模拟数据（开发环境）
	// 根据日期判断是否为工作日
	parsedDate, err := time.Parse("2006-01-02", workDate)
	if err != nil {
		return nil, fmt.Errorf("日期格式错误: %w", err)
	}

	// 简单的工作日判断：周一到周五
	weekday := parsedDate.Weekday()
	isWorkDay := weekday >= time.Monday && weekday <= time.Friday

	// 模拟考勤数据
	attendanceData := &AttendanceData{
		UserId:         employeeID,
		WorkDate:       workDate,
		WorkStartTime:  "09:00",
		WorkEndTime:    "18:00",
		IsWorkDay:      isWorkDay,
		AttendanceType: "normal",
	}

	log.Printf("返回模拟考勤数据: %+v", attendanceData)
	return attendanceData, nil
}
