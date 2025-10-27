package rules

import (
	"fmt"
	"log"
	"my-ai-app/model"
	"strings"
	"time"
)

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

	// (表单) 检查补卡时间
	if appData.ApplicationType == "补打卡" && appData.ApplicationTime > standardWorkTime {
		return &model.AnalysisResult{IsAbnormal: true, Reason: "迟到补卡"}
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

		// 规则 4.3: 类型验证 (AI 判断，逻辑不变)
		if !imageData.IsProofTypeValid {
			currentImageFailures = append(currentImageFailures, fmt.Sprintf("AI判定：证明材料类型[%s]与申请类型[%s]不符", imageData.RequestType, appData.ApplicationType))
		} else {
			currentValidation.TypeOK = true
			log.Printf("AI 判定：证据类型[%s]有效", imageData.RequestType)
		}

		// 规则 4.4: 时间验证 (仅补打卡，逻辑不变)
		if appData.ApplicationType == "补打卡" {
			currentValidation.TimeOK = false // 补打卡必须验证时间
			effectiveTime := imageData.RequestTime
			if imageData.TimeFromContent != "" && imageData.TimeFromContent != "未知" {
				effectiveTime = imageData.TimeFromContent
			}
			if effectiveTime == "未知" {
				currentImageFailures = append(currentImageFailures, "补卡证明材料未识别到具体时间")
			} else {
				isLater, err := compareTimes(effectiveTime, appData.ApplicationTime)
				if err != nil {
					currentImageFailures = append(currentImageFailures, fmt.Sprintf("时间格式错误: %v", err))
				} else if isLater {
					currentImageFailures = append(currentImageFailures, fmt.Sprintf("证明材料时间[%s]晚于申请补卡时间[%s]，无法证明在申请时间前就在公司", effectiveTime, appData.ApplicationTime))
				} else {
					currentValidation.TimeOK = true
					log.Printf("时间验证通过: 申请时间[%s], 证据时间[%s] (证据时间早于或等于申请时间)", appData.ApplicationTime, effectiveTime)
				}
			}
		}

		// --- 裁决当前图片 ---
		if len(currentImageFailures) == 0 && currentValidation.NameOK && currentValidation.DateOK && currentValidation.TypeOK && currentValidation.TimeOK {
			passedImageIndex = i + 1
			break
		} else {
			failureSummary := fmt.Sprintf("图片 %d 失败: (%s)", i+1, strings.Join(currentImageFailures, "；"))
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

		finalReason := fmt.Sprintf("正常 (图片 %d/%d 验证通过)%s", passedImageIndex, len(imageList), timeWarning)

		return &model.AnalysisResult{
			IsAbnormal: false,
			Reason:     finalReason,
		}
	} else {
		finalReason := "所有图片均未通过验证：" + strings.Join(allImageFailures, " | ")
		log.Println(finalReason)
		return &model.AnalysisResult{
			IsAbnormal: true,
			Reason:     finalReason,
		}
	}
}
