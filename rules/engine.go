package rules

import (
	"fmt"
	"log"
	"my-ai-app/model"
	"regexp"
	"strings"
	"time"
)

// normalizeTimeFormat 将各种时间格式转换为 HH:mm 格式
func normalizeTimeFormat(timeStr string) (string, error) {
	if timeStr == "" || timeStr == "未知" {
		return "", fmt.Errorf("时间字符串为空或未知")
	}

	// 尝试不同的时间格式
	layouts := []string{
		"15:04",               // HH:mm
		"2006-01-02 15:04:05", // Y-m-d H:i:s
		"2006-01-02 15:04",    // Y-m-d H:i
		"2006/01/02 15:04:05", // Y/m/d H:i:s
		"2006/01/02 15:04",    // Y/m/d H:i
		"01-02 15:04",         // m-d H:i
		"01/02 15:04",         // m/d H:i
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, timeStr); err == nil {
			return t.Format("15:04"), nil
		}
	}

	// 尝试正则表达式提取时间
	timeRegex := regexp.MustCompile(`(\d{1,2}):(\d{2})`)
	if matches := timeRegex.FindStringSubmatch(timeStr); len(matches) >= 3 {
		hour := matches[1]
		minute := matches[2]
		if len(hour) == 1 {
			hour = "0" + hour
		}
		return hour + ":" + minute, nil
	}

	return "", fmt.Errorf("无法解析时间格式: %s", timeStr)
}

func compareTimes(timeStr1, timeStr2 string) (bool, error) {
	layout := "15:04"
	t1, err1 := time.Parse(layout, timeStr1)
	if err1 != nil {
		return false, fmt.Errorf("无法解析时间 '%s': %w", timeStr1, err1)
	}
	t2, err2 := time.Parse(layout, timeStr2)
	if err2 != nil {
		return false, fmt.Errorf("无法解析时间 '%s': %w", timeStr2, err2)
	}

	return t1.After(t2), nil
}

func ValidateApplication(appData model.ApplicationData, oaAttendance *model.OaAttendanceData, imageList []*model.ExtractedData) *model.AnalysisResult {
	// todo 接入oa后，根据实际工作时间设置对应的弹性上班时间
	const standardWorkTime = "09:00"

	// 确定申请的时间范围
	var startTime, endTime string
	if appData.StartTime != "" || appData.EndTime != "" {
		// 提供了新字段（start_time 或 end_time）
		startTime = appData.StartTime
		endTime = appData.EndTime
		if startTime != "" && endTime != "" {
			log.Printf("检测到上下班卡同时申请 - 上班时间: %s, 下班时间: %s", startTime, endTime)
		} else if startTime != "" {
			log.Printf("检测到上班卡申请 - 上班时间: %s", startTime)
		} else {
			log.Printf("检测到下班卡申请 - 下班时间: %s", endTime)
		}
	} else if appData.ApplicationTime != "" {
		// 向后兼容：单个时间申请
		startTime = appData.ApplicationTime
		endTime = ""
		log.Printf("检测到单个时间申请 - 申请时间: %s", startTime)
	} else {
		return &model.AnalysisResult{IsAbnormal: true, Reason: "未提供申请时间"}
	}

	// 先进行“已有打卡则无需补卡”的快速判断
	if appData.ApplicationType == "补打卡" && len(appData.AttendanceInfo) > 0 {
		// 统一到 HH:mm 并去重
		normalize := func(ts []string) []string {
			out := make([]string, 0, len(ts))
			seen := map[string]struct{}{}
			for _, t := range ts {
				if nt, err := normalizeTimeFormat(strings.TrimSpace(t)); err == nil {
					if _, ok := seen[nt]; !ok {
						seen[nt] = struct{}{}
						out = append(out, nt)
					}
				}
			}
			return out
		}

		clockTimes := normalize(appData.AttendanceInfo)
		var target string
		if appData.StartTime != "" {
			if nt, err := normalizeTimeFormat(appData.StartTime); err == nil {
				target = nt
			}
		}
		if appData.EndTime != "" { // 如果是下班卡，优先生效
			if nt, err := normalizeTimeFormat(appData.EndTime); err == nil {
				target = nt
			}
		}

		if target != "" && len(clockTimes) > 0 {
			// 查找最接近 target 的已有打卡点，判断是否已覆盖申请
			// 规则：
			// - 上班卡：存在 <= target 的打卡则认为已有打卡，无需补卡
			// - 下班卡：存在 >= target 的打卡则认为已有打卡，无需补卡
			isStart := appData.EndTime == ""
			for _, ct := range clockTimes {
				if isStart {
					later, _ := compareTimes(ct, target)
					if !later {
						return &model.AnalysisResult{IsAbnormal: true, Reason: fmt.Sprintf("已有打卡记录%s，无需补卡", ct)}
					}
				} else {
					earlier, _ := compareTimes(target, ct)
					if !earlier {
						return &model.AnalysisResult{IsAbnormal: true, Reason: fmt.Sprintf("已有打卡记录%s，无需补卡", ct)}
					}
				}
			}
		}
	}

	// 检查是否缺少必要的图片
	if (appData.ApplicationType == "病假" || appData.ApplicationType == "补打卡") && len(imageList) == 0 {
		if len(appData.ImageUrls) == 0 {
			return &model.AnalysisResult{IsAbnormal: true, Reason: "缺少必要的证明材料图片 (未提供 image_urls)"}
		} else {
			return &model.AnalysisResult{IsAbnormal: true, Reason: "所有图片均处理失败，无法验证"}
		}
	}

	allImageFailures := make([]string, 0, len(imageList))
	passedImageIndex := -1

	for i, imageData := range imageList {
		if imageData == nil {
			continue
		} // 跳过处理失败的图片占位符

		var currentImageFailures []string
		log.Printf("--- 正在验证图片 %d/%d ---", i+1, len(imageList))

		currentValidation := struct{ NameOK, DateOK, TypeOK, TimeOK bool }{
			NameOK: true, DateOK: false, TypeOK: false, TimeOK: true,
		}

		// 规则 4.1: 姓名验证 (逻辑不变)
		if appData.ApplicationType == "病假" {
			currentValidation.NameOK = false
			if appData.Alias == "" {
				currentImageFailures = append(currentImageFailures, "病假申请未提供申请人姓名")
			} else if imageData.ExtractedName == "未知" {
				currentImageFailures = append(currentImageFailures, "证明材料未体现申请人姓名")
			} else if !strings.Contains(imageData.ExtractedName, appData.Alias) &&
				!strings.Contains(appData.Alias, imageData.ExtractedName) {
				currentImageFailures = append(currentImageFailures, fmt.Sprintf("证明材料姓名[%s]与申请人[%s]不符", imageData.ExtractedName, appData.Alias))
			} else {
				currentValidation.NameOK = true
				log.Println("姓名验证通过")
			}
		} else {
			log.Println("非病假申请，跳过姓名强制验证")
		}

		// 规则 4.2: 日期验证 (逻辑不变)
		if imageData.RequestDate == "未知" {
			currentImageFailures = append(currentImageFailures, "证明材料未识别到日期")
		} else if appData.ApplicationDate != imageData.RequestDate {
			currentImageFailures = append(currentImageFailures, fmt.Sprintf("证明材料日期[%s]与申请日期[%s]不符", imageData.RequestDate, appData.ApplicationDate))
		} else {
			currentValidation.DateOK = true
			log.Printf("日期验证通过: 申请日期[%s] = 图片日期[%s]", appData.ApplicationDate, imageData.RequestDate)
		}

		// 规则 4.3: 类型验证
		if !imageData.IsProofTypeValid {
			rt := imageData.RequestType
			if rt == "" || rt == "未知" {
				rt = "无法识别的类型"
			}

			// 补打卡兜底纠偏：饭堂/消费/账单类直接视为有效
			if appData.ApplicationType == "补打卡" {
				lower := strings.ToLower(rt)
				if strings.Contains(lower, "账单") || strings.Contains(lower, "消费") || strings.Contains(lower, "饭堂") || strings.Contains(lower, "食堂") || strings.Contains(lower, "小票") || strings.Contains(lower, "收银") || strings.Contains(lower, "票据") || strings.Contains(lower, "订单") || strings.Contains(lower, "支付") || strings.Contains(lower, "交易") || strings.Contains(lower, "就餐") || strings.Contains(lower, "餐饮") || strings.Contains(lower, "餐费") || strings.Contains(lower, "餐卡") {
					currentValidation.TypeOK = true
					log.Printf("类型纠偏：[%s] 视为补打卡有效证据", rt)
					// 纠偏后跳过类型失败记录，避免重复日志
					goto SKIP_TYPE_FAILURE
				}
			}

			var reason string
			if appData.ApplicationType == "补打卡" && rt == "聊天记录" {
				// 对于聊天记录，先记录类型验证失败，等时间验证完成后再决定最终提示语
				reason = "补打卡证明需提供有力清晰的在司真实证明，如 ①饭堂消费记录； ②电脑开机时间；③网页浏览或文件处理记录，当前是【聊天记录】"
			} else {
				reason = fmt.Sprintf("证据类型无效：检测为[%s]，与申请类型[%s]不匹配", rt, appData.ApplicationType)
			}
			currentImageFailures = append(currentImageFailures, reason)
			log.Printf("类型验证未通过: %s", reason)
		} else {
			currentValidation.TypeOK = true
			log.Printf("AI 判定：证据类型[%s]有效", imageData.RequestType)
			// 聊天记录特殊提示
			if appData.ApplicationType == "补打卡" && imageData.RequestType == "聊天记录" {
				log.Printf("图片类型: 聊天记录 (仍进行时间校验)")
			}
		}

	SKIP_TYPE_FAILURE:

		// 时间点验证
		if appData.ApplicationType == "补打卡" {
			currentValidation.TimeOK = false
			// 优先使用AI返回的候选列表
			candidates := make([]string, 0, 8)
			if len(imageData.CandidateTimes) > 0 {
				// AI已经根据申请时间智能筛选，直接使用
				candidates = append(candidates, imageData.CandidateTimes...)
				log.Printf("时间候选: AI智能筛选=%v", imageData.CandidateTimes)
			} else {
				// AI未返回候选时间，使用单值字段
				if imageData.RequestTime != "" && imageData.RequestTime != "未知" {
					candidates = append(candidates, imageData.RequestTime)
				}
				if imageData.TimeFromContent != "" && imageData.TimeFromContent != "未知" {
					candidates = append(candidates, imageData.TimeFromContent)
				}
				log.Printf("时间候选: request_time='%s', time_from_content='%s'", imageData.RequestTime, imageData.TimeFromContent)
			}

			// 选择规则：上班卡取最早；下班卡取最晚；区间优先取区间内，否则取最接近
			pickExtremum := func(times []string, pickMax bool) (string, error) {
				best := ""
				for _, t := range times {
					nt, err := normalizeTimeFormat(t)
					if err != nil {
						continue
					}
					if best == "" {
						best = nt
						continue
					}
					isAfter, _ := compareTimes(nt, best)
					if pickMax {
						if isAfter {
							best = nt
						}
					} else {
						if !isAfter {
							best = nt
						}
					}
				}
				if best == "" {
					return "", fmt.Errorf("无有效时间")
				}
				return best, nil
			}

			var effectiveTime string
			if len(imageData.CandidateTimes) > 0 {
				// AI已经智能筛选，直接使用第一个（最合适的）
				if len(candidates) > 0 {
					if nt, err := normalizeTimeFormat(candidates[0]); err == nil {
						effectiveTime = nt
					} else {
						effectiveTime = "未知"
					}
				} else {
					effectiveTime = "未知"
				}
				log.Printf("选用时间: '%s' (AI智能筛选)", effectiveTime)
			} else {
				// AI未筛选，使用原有逻辑
				if startTime != "" && endTime == "" {
					// 上班卡：取最早
					if t, err := pickExtremum(candidates, false); err == nil {
						effectiveTime = t
					} else {
						effectiveTime = "未知"
					}
				} else if startTime == "" && endTime != "" {
					// 下班卡：取最晚
					if t, err := pickExtremum(candidates, true); err == nil {
						effectiveTime = t
					} else {
						effectiveTime = "未知"
					}
				} else {
					// 区间：优先取区间内
					normalized := make([]string, 0, len(candidates))
					for _, t := range candidates {
						if nt, err := normalizeTimeFormat(t); err == nil {
							normalized = append(normalized, nt)
						}
					}
					inRange := func(nt string) bool {
						sOK, _ := compareTimes(startTime, nt) // start > nt ?
						eOK, _ := compareTimes(nt, endTime)   // nt > end ?
						return !sOK && !eOK
					}
					for _, nt := range normalized {
						if inRange(nt) {
							effectiveTime = nt
							break
						}
					}
					if effectiveTime == "" {
						// 退化策略：若全部 < start 取最大；若全部 > end 取最小
						if t, err := pickExtremum(normalized, false); err == nil {
							effectiveTime = t
						}
					}
					if effectiveTime == "" {
						effectiveTime = "未知"
					}
					log.Printf("选用时间: '%s' (start='%s', end='%s')", effectiveTime, startTime, endTime)
				}
			}

			if effectiveTime == "未知" {
				msg := "未识别到有效时间（图片可能无时间信息或无法解析）"
				currentImageFailures = append(currentImageFailures, msg)
				log.Printf("时间验证未通过: %s", msg)
			} else {
				// 标准化时间格式
				normalizedTime, err := normalizeTimeFormat(effectiveTime)
				if err != nil {
					msg := fmt.Sprintf("时间格式错误（无法解析为HH:mm）: %v", err)
					currentImageFailures = append(currentImageFailures, msg)
					log.Printf("时间验证未通过: %s", msg)
				} else {
					// 根据申请类型进行时间验证
					if startTime != "" && endTime != "" {
						// 上下班卡同时申请：图片时间必须在申请时间范围内
						startValid, startErr := compareTimes(startTime, normalizedTime)
						endValid, endErr := compareTimes(normalizedTime, endTime)
						if startErr != nil || endErr != nil {
							currentImageFailures = append(currentImageFailures, fmt.Sprintf("时间比较错误: %v", startErr))
						} else if startValid && endValid {
							currentValidation.TimeOK = true
							log.Printf("时间验证通过: 申请时间范围[%s-%s], 证据时间[%s] (在范围内)", startTime, endTime, normalizedTime)
						} else {
							if !startValid {
								msg := fmt.Sprintf("证据时间[%s]晚于上班时间[%s]，不满足上班卡≤规则", normalizedTime, startTime)
								currentImageFailures = append(currentImageFailures, msg)
								log.Printf("时间验证未通过: %s", msg)
							} else {
								msg := fmt.Sprintf("证据时间[%s]早于下班时间[%s]，不满足下班卡≥规则", normalizedTime, endTime)
								currentImageFailures = append(currentImageFailures, msg)
								log.Printf("时间验证未通过: %s", msg)
							}
						}
					} else if startTime != "" {
						// 只有上班时间：图片时间必须早于或等于上班时间
						isLater, err := compareTimes(normalizedTime, startTime)
						if err != nil {
							msg := fmt.Sprintf("时间比较错误: %v", err)
							currentImageFailures = append(currentImageFailures, msg)
							log.Printf("时间验证未通过: %s", msg)
						} else if isLater {
							msg := fmt.Sprintf("证据时间[%s]晚于上班时间[%s]，不满足上班卡≤规则", normalizedTime, startTime)
							currentImageFailures = append(currentImageFailures, msg)
							log.Printf("时间验证未通过: %s", msg)
							// 如果是聊天记录，更新提示语
							if imageData.RequestType == "聊天记录" {
								for j, failure := range currentImageFailures {
									if strings.Contains(failure, "补打卡证明需提供有力清晰的在司真实证明") {
										currentImageFailures[j] = "时间验证不通过，补打卡证明需提供有力清晰的在司真实证明，如 ①饭堂消费记录； ②电脑开机时间；③网页浏览或文件处理记录，当前是【聊天记录】"
										break
									}
								}
							}
						} else {
							currentValidation.TimeOK = true
							log.Printf("时间验证通过: 上班申请时间[%s], 证据时间[%s] (证据时间早于或等于上班时间)", startTime, normalizedTime)
							// 如果是聊天记录，更新提示语
							if imageData.RequestType == "聊天记录" {
								for j, failure := range currentImageFailures {
									if strings.Contains(failure, "补打卡证明需提供有力清晰的在司真实证明") {
										currentImageFailures[j] = "时间验证通过，补打卡证明需提供有力清晰的在司真实证明，如 ①饭堂消费记录； ②电脑开机时间；③网页浏览或文件处理记录，当前是【聊天记录】"
										break
									}
								}
							}
						}
					} else if endTime != "" {
						// 只有下班时间：图片时间必须晚于或等于下班时间
						isEarlier, err := compareTimes(endTime, normalizedTime)
						if err != nil {
							msg := fmt.Sprintf("时间比较错误: %v", err)
							currentImageFailures = append(currentImageFailures, msg)
							log.Printf("时间验证未通过: %s", msg)
						} else if isEarlier {
							msg := fmt.Sprintf("证据时间[%s]早于下班时间[%s]，不满足下班卡≥规则", normalizedTime, endTime)
							currentImageFailures = append(currentImageFailures, msg)
							log.Printf("时间验证未通过: %s", msg)
							// 如果是聊天记录，更新提示语
							if imageData.RequestType == "聊天记录" {
								for j, failure := range currentImageFailures {
									if strings.Contains(failure, "补打卡证明需提供有力清晰的在司真实证明") {
										currentImageFailures[j] = "时间验证不通过，补打卡证明需提供有力清晰的在司真实证明，如 ①饭堂消费记录； ②电脑开机时间；③网页浏览或文件处理记录，当前是【聊天记录】"
										break
									}
								}
							}
						} else {
							currentValidation.TimeOK = true
							log.Printf("时间验证通过: 下班申请时间[%s], 证据时间[%s] (证据时间晚于或等于下班时间)", endTime, normalizedTime)
							// 如果是聊天记录，更新提示语
							if imageData.RequestType == "聊天记录" {
								for j, failure := range currentImageFailures {
									if strings.Contains(failure, "补打卡证明需提供有力清晰的在司真实证明") {
										currentImageFailures[j] = "时间验证通过，补打卡证明需提供有力清晰的在司真实证明，如 ①饭堂消费记录； ②电脑开机时间；③网页浏览或文件处理记录，当前是【聊天记录】"
										break
									}
								}
							}
						}
					}
				}
			}
		}

		// --- 裁决当前图片 ---
		if len(currentImageFailures) == 0 && currentValidation.NameOK && currentValidation.DateOK && currentValidation.TypeOK && currentValidation.TimeOK {
			passedImageIndex = i + 1
			break
		} else {
			failureSummary := fmt.Sprintf("图片验证失败: (%s)", strings.Join(currentImageFailures, "；"))
			log.Println(failureSummary)
			allImageFailures = append(allImageFailures, failureSummary)
		}
	}

	if passedImageIndex != -1 {
		log.Printf("图片 %d 验证通过！", passedImageIndex)

		// 构建时间警告信息（如果有OA考勤数据）
		var timeWarning string
		if oaAttendance != nil {
			timeWarning = fmt.Sprintf(" (OA考勤时间: %s-%s)", oaAttendance.StandardInTime, oaAttendance.StandardOutTime)
		}

		finalReason := fmt.Sprintf("时间检测通过 (图片信息 验证通过)%s", timeWarning)

		return &model.AnalysisResult{
			IsAbnormal: false,
			Reason:     finalReason,
		}
	} else {
		finalReason := strings.Join(allImageFailures, " | ")
		log.Println(finalReason)
		return &model.AnalysisResult{
			IsAbnormal: true,
			Reason:     finalReason,
		}
	}
}
