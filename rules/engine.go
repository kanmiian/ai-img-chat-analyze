package rules

import (
	"fmt"
	"log"
	"my-ai-app/model"
	"strings"
)

func ValidateApplication(appData model.ApplicationData, imageData *model.ExtractedData) *model.AnalysisResult {

	const standardWorkTime = "09:00"

	// 规则 2: (表单) 检查补卡时间
	if appData.ApplicationType == "补打卡" && appData.ApplicationTime > standardWorkTime {
		return &model.AnalysisResult{IsAbnormal: true, Reason: "迟到补卡"}
	}

	// 规则 3: (图片) 检查是否缺少必要的图片
	if (appData.ApplicationType == "病假" || appData.ApplicationType == "补打卡") && imageData == nil {
		return &model.AnalysisResult{
			IsAbnormal: true,
			Reason:     "缺少必要的证明材料图片",
		}
	}

	// 规则 4: (交叉验证) 如果有图片，开始验证
	if imageData != nil {
		// 使用传入的 alias 与图片中提取的姓名进行比对
		if appData.Alias != "" {
			log.Printf("交叉验证: 申请姓名[%s], 图片姓名[%s]", appData.Alias, imageData.ExtractedName)

			// 规则 4.1: 验证姓名
			// 检查申请人的姓名是否在图片提取的姓名中
			// (使用 strings.Contains 来容错，例如图片上是"张三"，申请人是"张三(研发部)")
			if imageData.ExtractedName != "未知" &&
				!strings.Contains(imageData.ExtractedName, appData.Alias) &&
				!strings.Contains(appData.Alias, imageData.ExtractedName) {

				return &model.AnalysisResult{
					IsAbnormal: true,
					Reason:     fmt.Sprintf("证明材料上的姓名[%s]与申请人[%s]不符", imageData.ExtractedName, appData.Alias),
				}
			}
		} else {
			// 没有传入 alias 时，只记录图片中识别的姓名，不进行验证
			log.Printf("图片识别姓名: %s (无申请姓名，跳过姓名验证)", imageData.ExtractedName)
		}

		// 规则 4.2: 验证日期
		if appData.ApplicationDate != imageData.RequestDate && imageData.RequestDate != "未知" {
			return &model.AnalysisResult{
				IsAbnormal: true,
				Reason:     "证明材料日期与申请日期不符",
			}
		}

		// 规则 4.3: 验证补卡时间
		if appData.ApplicationType == "补打卡" && appData.ApplicationTime <= standardWorkTime {
			if imageData.RequestTime == "未知" || imageData.RequestTime > appData.ApplicationTime {
				return &model.AnalysisResult{
					IsAbnormal: true,
					Reason:     fmt.Sprintf("证明材料(时间: %s)无法支持您在 %s 或之前已在工作", imageData.RequestTime, appData.ApplicationTime),
				}
			}
		}
	}

	// 默认: 所有规则都通过了
	return &model.AnalysisResult{
		IsAbnormal: false,
		Reason:     "正常",
	}
}
