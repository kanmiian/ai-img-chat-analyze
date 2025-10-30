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
        {URL: "https://oa.shiyuegame.com/aetherupload/display/file/20250213/015b626d257019a918ee252894763161.png/", Date: "2025-02-12", Time: "18:32", Type: "下班补打卡"},
        {URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251027/204c4a762af3a062c81399e5ba8e9bd9.png/", Date: "2025-10-20", Time: "08:55", Type: "上班补打卡"},
        {URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251027/9401662f9bce8be4752d93739452c55e.png/", Date: "2025-10-21", Time: "08:57", Type: "上班补打卡"},
        {URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251027/864d78b1e8ec586c4ec2db9a6ba4c13f.png/", Date: "2025-10-22", Time: "09:18", Type: "上班补打卡"},
        {URL: "https://oa.shiyuegame.com/aetherupload/display/file/20251028/974ba75649bf75cfd451c80819f784aa.png/", Date: "2025-10-11", Time: "19:13", Type: "下班补打卡"},
    }

    var out []RequestPayload
    user := "sy-test"
    alias := "测试用户"
    for k, b := range bases {
        // 使用准确的日期和时间，不做偏移
        start := ""
        end := ""
        if b.Type == "下班补打卡" {
            end = b.Time
        } else {
            start = b.Time
        }
        payload := RequestPayload{
            UserId:              fmt.Sprintf("%s-b%d", user, k+1),
            Alias:               alias,
            ApplicationType:     b.Type,
            ApplicationDate:     b.Date,
            StartTime:           start,
            EndTime:             end,
            ApplicationTime:     "",
            ImageUrls:           []string{b.URL},
            AttendanceInfo:      []string{},
            NeedImageValidation: true,
        }
        out = append(out, payload)
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
		server = "http://localhost:3000"
	}
	// 若未包含协议前缀，补齐 http://
	if !(len(server) >= 7 && (server[:7] == "http://" || (len(server) >= 8 && server[:8] == "https://"))) {
		server = "http://" + server
	}
	endpoint := fmt.Sprintf("%s/api/v1/check-by-volcano", server)
	cases := generateCases()
	baseURLIndex := map[string]int{
		"https://oa.shiyuegame.com/aetherupload/display/file/20250213/015b626d257019a918ee252894763161.png/": 1,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251027/204c4a762af3a062c81399e5ba8e9bd9.png/": 2,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251027/9401662f9bce8be4752d93739452c55e.png/": 3,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251027/864d78b1e8ec586c4ec2db9a6ba4c13f.png/": 4,
		"https://oa.shiyuegame.com/aetherupload/display/file/20251028/974ba75649bf75cfd451c80819f784aa.png/": 5,
	}
    log.Printf("开始执行测试用例，总计: %d，目标: %s", len(cases), endpoint)
    totalRounds := 10
    totalRequests := 0
    approveCount := 0
    fail := 0
    for round := 1; round <= totalRounds; round++ {
        log.Printf("-- Round %d/%d --", round, totalRounds)
        for idx, c := range cases {
            status, resp, err := postJSON(endpoint, c)
            totalRequests++
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
            // 解析统一返回
            var res struct {
                Approve   bool   `json:"approve"`
                Valid     bool   `json:"valid"`
                DateMatch bool   `json:"date_match"`
                TimeMatch bool   `json:"time_match"`
                Reason    string `json:"reason"`
                Message   string `json:"message"`
            }
            if err := json.Unmarshal(resp, &res); err != nil {
                fail++
                log.Printf("[%02d] ✗ 解析返回失败: %v, 原文: %s", idx+1, err, string(resp))
                continue
            }
            if res.Approve {
                approveCount++
            }
            imgIdx := baseURLIndex[c.ImageUrls[0]]
            ts := func() string { if c.StartTime != "" { return c.StartTime } else { return c.EndTime } }()
            log.Printf("[img=%d][%02d] %s %s %s | approve=%v date_match=%v time_match=%v | reason=%s",
                imgIdx, idx+1, c.ApplicationType, c.ApplicationDate, ts, res.Approve, res.DateMatch, res.TimeMatch, res.Reason)
        }
    }
    ratio := 100.0 * float64(approveCount) / math.Max(1, float64(totalRequests))
    log.Printf("完成: 总请求 %d, 失败 %d, approve率 %.1f%%", totalRequests, fail, ratio)
}
