package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"time"
)

type RequestPayload struct {
	UserId              string   `json:"user_id"`
	Alias               string   `json:"alias"`
	ApplicationType     string   `json:"application_type"`
	ApplicationDate     string   `json:"application_date"`
	StartTime           string   `json:"start_time"`
	EndTime             string   `json:"end_time"`
	ApplicationTime     string   `json:"application_time"`
	ImageUrls           []string `json:"image_urls"`
	AttendanceInfo      []string `json:"attendance_info"`
	NeedImageValidation bool     `json:"need_image_validation"`
}

type Base struct {
	URL  string
	Date string // yyyy-mm-dd
	Time string // HH:mm
	Type string // "上班补打卡" 或 "下班补打卡"
}

func mustParseDate(s string) time.Time {
	layouts := []string{"2006-1-2", "2006-01-02", "2006/01/02"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	log.Fatalf("无法解析日期: %s", s)
	return time.Time{}
}

func mustParseTime(s string) (int, int) {
	layouts := []string{"15:04", "15:4", "3:04PM"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Hour(), t.Minute()
		}
	}
	log.Fatalf("无法解析时间: %s", s)
	return 0, 0
}

func shiftDate(t time.Time, delta int) string {
	nt := t.AddDate(0, 0, delta)
	return nt.Format("2006-01-02")
}

func shiftTime(h, m, deltaMin int) string {
	t := time.Date(2000, 1, 1, h, m, 0, 0, time.Local)
	nt := t.Add(time.Duration(deltaMin) * time.Minute)
	return nt.Format("15:04")
}

func generateCases() []RequestPayload {
	bases := []Base{
		{URL: "https://oa.shiyuegame.com/aetherupload/display/file/20250213/015b626d257019a918ee252894763161.png/", Date: "2015-2-12", Time: "18:32", Type: "下班补打卡"},
		{URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251027/204c4a762af3a062c81399e5ba8e9bd9.png/", Date: "2025-10-20", Time: "08:55", Type: "上班补打卡"},
		{URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251027/9401662f9bce8be4752d93739452c55e.png/", Date: "2025-10-21", Time: "08:57", Type: "上班补打卡"},
		{URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251027/864d78b1e8ec586c4ec2db9a6ba4c13f.png/", Date: "2025-10-22", Time: "09:18", Type: "上班补打卡"},
		{URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251028/974ba75649bf75cfd451c80819f784aa.png/", Date: "2025-10-11", Time: "19:13", Type: "下班补打卡"},
	}

	var out []RequestPayload
	user := "sy-test"
	alias := "测试用户"
	// 每张图片生成的用例数量，合计约 25 条
	perBase := 5
	for k, b := range bases {
		d := mustParseDate(b.Date)
		hh, mm := mustParseTime(b.Time)
		deltasD := []int{0, -1, 1, -2, 2}
		deltasM := []int{0, -5, 5, -10, 10}
		count := 0
		for i, dd := range deltasD {
			for j, dm := range deltasM {
				ad := shiftDate(d, dd)
				at := shiftTime(hh, mm, dm)
				payload := RequestPayload{
					UserId:              fmt.Sprintf("%s-b%d-%d-%d", user, k+1, i, j),
					Alias:               alias,
					ApplicationType:     b.Type,
					ApplicationDate:     ad,
					StartTime:           map[bool]string{true: ""}[b.Type == "下班补打卡"],
					EndTime:             map[bool]string{true: at}[b.Type == "下班补打卡"],
					ApplicationTime:     "",
					ImageUrls:           []string{b.URL},
					AttendanceInfo:      []string{},
					NeedImageValidation: true,
				}
				out = append(out, payload)
				count++
				if count >= perBase {
					break
				}
			}
			if count >= perBase {
				break
			}
		}
	}
	// 截断到最多 25 条
	if len(out) > 25 {
		out = out[:25]
	}
	return out
}

func postJSON(url string, body any) (int, []byte, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(buf))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

func main() {
	server := os.Getenv("TEST_SERVER")
	if server == "" {
		server = "http://localhost:8080"
	}
	// 若未包含协议前缀，补齐 http://
	if !(len(server) >= 7 && (server[:7] == "http://" || (len(server) >= 8 && server[:8] == "https://"))) {
		server = "http://" + server
	}
	endpoint := fmt.Sprintf("%s/api/v1/analyze-volcano", server)
	cases := generateCases()
	baseURLIndex := map[string]int{
		"https://oa.shiyuegame.com/aetherupload/display/file/20250213/015b626d257019a918ee252894763161.png/": 1,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251027/204c4a762af3a062c81399e5ba8e9bd9.png/": 2,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251027/9401662f9bce8be4752d93739452c55e.png/": 3,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251027/864d78b1e8ec586c4ec2db9a6ba4c13f.png/": 4,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251028/974ba75649bf75cfd451c80819f784aa.png/": 5,
	}
	log.Printf("开始执行测试用例，总计: %d，目标: %s", len(cases), endpoint)
	ok, fail := 0, 0
	for idx, c := range cases {
		status, resp, err := postJSON(endpoint, c)
		if err != nil {
			fail++
			log.Printf("[%02d] 请求失败: %v", idx+1, err)
			continue
		}
		if status < 200 || status >= 300 {
			fail++
			log.Printf("[%02d] ✗ HTTP %d: %s", idx+1, status, string(resp))
			continue
		}
		// 解析 AnalysisResult，获取第一张图片的 ExtractedData
		var ar struct {
			IsAbnormal     bool   `json:"is_abnormal"`
			Reason         string `json:"reason"`
			ImagesAnalysis []struct {
				Success       bool `json:"success"`
				ExtractedData *struct {
					Approve   bool   `json:"approve"`
					DateMatch bool   `json:"date_match"`
					TimeMatch bool   `json:"time_match"`
					ReasonLLM string `json:"reason"`
				} `json:"extracted_data"`
			} `json:"images_analysis"`
		}
		if err := json.Unmarshal(resp, &ar); err != nil {
			fail++
			log.Printf("[%02d] ✗ 解析返回失败: %v, 原文: %s", idx+1, err, string(resp))
			continue
		}
		// 取第一条有效图片结果
		var approve, dateMatch, timeMatch bool
		reason := ar.Reason
		if len(ar.ImagesAnalysis) > 0 && ar.ImagesAnalysis[0].ExtractedData != nil {
			ed := ar.ImagesAnalysis[0].ExtractedData
			approve = ed.Approve
			dateMatch = ed.DateMatch
			timeMatch = ed.TimeMatch
			if ed.ReasonLLM != "" {
				reason = ed.ReasonLLM
			}
		}
		// 计算图片序号标识
		imgIdx := baseURLIndex[c.ImageUrls[0]]
		ok++
		log.Printf("[img=%d][%02d] %s %s %s | approve=%v date_match=%v time_match=%v | reason=%s",
			imgIdx, idx+1, c.ApplicationType, c.ApplicationDate, func() string { if c.StartTime != "" { return c.StartTime } else { return c.EndTime } }(), approve, dateMatch, timeMatch, reason)
	}
	ratio := 100.0 * float64(ok) / math.Max(1, float64(len(cases)))
	log.Printf("完成: 成功 %d, 失败 %d, 成功率 %.1f%%", ok, fail, ratio)
}
