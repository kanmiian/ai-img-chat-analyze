package rules

import (
	"fmt"
	"log"
	"my-ai-app/model"
	"strings"
)

func ValidateApplication(appData model.ApplicationData, imageData *model.ExtractedData) *model.AnalysisResult {

	// todo 接入oa后，根据实际工作时间设置对应的弹性上班时间
	const standardWorkTime = "09:00"

	// (表单) 检查补卡时间
	if appData.ApplicationType == "补打卡" && appData.ApplicationTime > standardWorkTime {
		return &model.AnalysisResult{IsAbnormal: true, Reason: "迟到补卡"}
	}

	// 检查是否缺少必要的图片
	if (appData.ApplicationType == "病假" || appData.ApplicationType == "补打卡") && imageData == nil {
		return &model.AnalysisResult{
			IsAbnormal: true,
			Reason:     "缺少必要的证明材料图片",
		}
	}

	validationFlags := struct {
		NameOK bool
		DateOK bool
		TypeOK bool
		TimeOK bool
	}{
		NameOK: false,
		DateOK: false,
		TypeOK: false,
		TimeOK: false,
	}
	var failureReasons []string

	//  如果有图片，开始验证
	if imageData != nil {

		if appData.ApplicationType == "病假" {
			// 病假，必须验证姓名
			if appData.Alias == "" {
				failureReasons = append(failureReasons, "病假申请未提供申请人姓名")
			} else if imageData.ExtractedName == "未知" {
				failureReasons = append(failureReasons, "证明材料未体现申请人姓名")
			} else if !strings.Contains(imageData.ExtractedName, appData.Alias) &&
				!strings.Contains(appData.Alias, imageData.ExtractedName) {
				failureReasons = append(failureReasons, fmt.Sprintf("证明材料姓名[%s]与申请人[%s]不符", imageData.ExtractedName, appData.Alias))
			} else {
				validationFlags.NameOK = true
				log.Println("姓名验证通过")
			}
		} else {
			validationFlags.NameOK = true
			log.Println("非病假申请，跳过姓名强制验证")
		}

		if imageData.RequestDate == "未知" {
			failureReasons = append(failureReasons, "证明材料未识别到日期")
		} else if appData.ApplicationDate != imageData.RequestDate {
			failureReasons = append(failureReasons, fmt.Sprintf("证明材料日期[%s]与申请日期[%s]不符", imageData.RequestDate, appData.ApplicationDate))
		} else {
			validationFlags.DateOK = true
			log.Printf("日期验证通过: 申请日期[%s] = 图片日期[%s]", appData.ApplicationDate, imageData.RequestDate)
		}

		if !imageData.IsProofTypeValid {
			failureReasons = append(failureReasons, fmt.Sprintf("AI判定：证明材料类型[%s]与申请类型[%s]不符", imageData.RequestType, appData.ApplicationType))
		} else {
			validationFlags.TypeOK = true
			log.Printf("AI 判定：证据类型[%s]有效", imageData.RequestType)
		}

		if appData.ApplicationType == "补打卡" && appData.ApplicationTime <= standardWorkTime {
			if imageData.RequestTime == "未知" {
				failureReasons = append(failureReasons, "补卡证明材料未识别到具体时间")
			} else if imageData.RequestTime > appData.ApplicationTime {
				failureReasons = append(failureReasons, fmt.Sprintf("证明材料时间[%s]无法支持在 %s 前工作", imageData.RequestTime, appData.ApplicationTime))
			} else {
				validationFlags.TimeOK = true
				log.Println("补卡时间验证通过")
			}
		} else {
			validationFlags.TimeOK = true
		}

	}

	if len(failureReasons) > 0 {

		var reasonParts []string

		// 检查通过的项
		if validationFlags.DateOK && validationFlags.TypeOK {
			reasonParts = append(reasonParts, "日期和类型无误")
		} else if validationFlags.DateOK {
			reasonParts = append(reasonParts, "日期无误")
		} else if validationFlags.TypeOK {
			reasonParts = append(reasonParts, "类型无误")
		}

		// 附加失败的项
		reasonParts = append(reasonParts, strings.Join(failureReasons, "，"))

		// 组合成最终原因
		finalReason := strings.Join(reasonParts, "；但")

		return &model.AnalysisResult{
			IsAbnormal: true,
			Reason:     finalReason,
		}
	}
	// 默认: 所有规则都通过了
	log.Println("所有验证通过，裁决为：正常")
	return &model.AnalysisResult{
		IsAbnormal: false,
		Reason:     "正常",
	}
}
