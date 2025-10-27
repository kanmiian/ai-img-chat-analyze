#!/bin/bash

# 火山引擎测试接口调用示例
# 用于OA系统调用火山引擎进行图片检查

echo "=== 火山引擎测试接口调用示例 ==="

# 测试数据
USER_ID="EMP001"
ALIAS="张三"
APPLICATION_TYPE="补打卡"
APPLICATION_TIME="09:05"
APPLICATION_DATE="2025-01-24"
REASON="忘记打卡"
IMAGE_URL1="https://oa.shiyuegame.com/aetherupload/display/file/20250124/e1cf7a84900ad106b2ddd936abced53e.png/"
IMAGE_URL2="https://oa.shiyuegame.com/aetherupload/display/file/20250124/02bd29b81fdcd335958dcca17a608a1f.jpg/"

echo "调用测试接口..."
echo "员工ID: $USER_ID"
echo "员工姓名: $ALIAS"
echo "申请类型: $APPLICATION_TYPE"
echo "申请时间: $APPLICATION_TIME"
echo "申请日期: $APPLICATION_DATE"
echo "申请原因: $REASON"
echo "图片数量: 2张"
echo ""

# 调用接口
curl -X POST http://localhost:8080/api/v1/test-volcano \
  -F "user_id=$USER_ID" \
  -F "alias=$ALIAS" \
  -F "application_type=$APPLICATION_TYPE" \
  -F "application_time=$APPLICATION_TIME" \
  -F "application_date=$APPLICATION_DATE" \
  -F "reason=$REASON" \
  -F "image_urls[]=$IMAGE_URL1" \
  -F "image_urls[]=$IMAGE_URL2" \
  -H "Content-Type: multipart/form-data" | jq '.'

echo ""
echo "=== 接口说明 ==="
echo "接口地址: POST /api/v1/test-volcano"
echo "返回格式:"
echo "  success: true/false (是否通过)"
echo "  message: 通过/不通过的原因"
echo "  data: 详细数据"
echo ""
echo "=== 参数说明 ==="
echo "user_id: 员工ID (必填)"
echo "alias: 员工姓名 (必填)"
echo "application_type: 申请类型 (必填)"
echo "application_time: 申请时间 (可选)"
echo "application_date: 申请日期 (可选)"
echo "reason: 申请原因 (可选)"
echo "image_urls[]: 图片URL数组 (必填，至少一张)"
