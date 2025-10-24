# 图片 URL 支持说明

## 概述

Go 服务现在支持两种图片输入方式：
1. **直接文件上传** - 通过 `multipart/form-data` 上传图片文件
2. **图片 URL** - 提供图片 URL，Go 服务会自动下载并处理

## 工作流程

当使用图片 URL 时，Go 服务作为"中间人"：

```
OA 系统 → Go API (接收 image_url)
         ↓
    从 URL 下载图片到内存
         ↓
    压缩图片 (最大 1000x1000)
         ↓
    转换为 JPEG (质量 80)
         ↓
    Base64 编码
         ↓
    发送给 AI 引擎
```

## API 接口

### 1. Qwen 分析接口

**请求地址**: `POST /api/v1/analyze-qwen`

**Content-Type**: `multipart/form-data`

**参数**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| user_id | string | 是 | 员工 ID |
| alias | string | 是 | 员工姓名 |
| application_type | string | 是 | 申请类型（如：病假、补打卡） |
| application_time | string | 否 | 申请时间 (HH:mm) |
| application_date | string | 否 | 申请日期 (YYYY-MM-DD) |
| reason | string | 否 | 申请理由 |
| image | file | 否* | 图片文件 |
| image_url | string | 否* | 图片 URL |

*注意：`image` 和 `image_url` 二选一，不能同时提供

### 2. Volcano 分析接口

**请求地址**: `POST /api/v1/analyze-volcano`

参数与 Qwen 接口相同。

## 使用示例

### 方式一：直接文件上传

```bash
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "application_date=2025-10-24" \
  -F "application_time=09:00" \
  -F "image=@/path/to/image.png"
```

### 方式二：使用图片 URL

```bash
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "application_date=2025-10-24" \
  -F "application_time=09:00" \
  -F "image_url=http://oa.company.com/upload-file/e025bbcd37180d1b75d95de2e533303a.png"
```

### JavaScript 示例

```javascript
// 使用图片 URL
const formData = new FormData();
formData.append('user_id', '12345');
formData.append('alias', '张三');
formData.append('application_type', '病假');
formData.append('application_date', '2025-10-24');
formData.append('application_time', '09:00');
formData.append('image_url', 'http://oa.company.com/upload-file/e025bbcd37180d1b75d95de2e533303a.png');

fetch('http://localhost:8080/api/v1/analyze-qwen', {
  method: 'POST',
  body: formData
})
.then(response => response.json())
.then(data => console.log(data))
.catch(error => console.error('Error:', error));
```

## 响应格式

```json
{
  "is_abnormal": false,
  "reason": "正常",
  "time_validation": {
    "is_valid": true,
    "is_work_day": true,
    "is_late": false,
    "risk_level": "low",
    "suggestion": "申请时间合理",
    "details": "申请时间在工作时间内"
  },
  "raw_text": "AI 提取的原始文本"
}
```

## 图片处理细节

### 1. 下载超时
- 默认超时：10 秒
- 适用于内网环境，下载速度快

### 2. 图片压缩
- 最大尺寸：1000x1000 像素
- 保持宽高比
- 使用 Lanczos3 算法（高质量）

### 3. 格式转换
- 输出格式：JPEG
- 压缩质量：80
- 支持输入格式：JPEG, PNG, GIF, BMP, WebP

### 4. 文件大小限制
- 下载前：无限制（但受超时限制）
- 处理后：自动压缩到合理大小

## 错误处理

### 常见错误

1. **图片 URL 无法访问**
```json
{
  "error": "Qwen 分析失败",
  "details": "图片处理失败: 无法下载图片 URL: Get \"...\": context deadline exceeded"
}
```

2. **图片和 URL 同时提供**
```json
{
  "error": "请求无效",
  "details": "图片URL和图片不能同时上传"
}
```

3. **图片格式不支持**
```json
{
  "error": "Qwen 分析失败",
  "details": "图片处理失败: 无法解码图片: image: unknown format"
}
```

## 性能优化建议

### 1. 内网部署
- Go 服务和 OA 系统部署在同一内网
- 图片下载速度快，通常 < 100ms

### 2. 图片大小
- 建议 OA 系统上传时就进行适当压缩
- 避免超大图片（> 10MB）

### 3. 并发处理
- Go 服务支持并发请求
- 建议控制并发数量，避免内存占用过高

## 安全考虑

### 1. URL 白名单
如果需要限制可访问的 URL，可以在 `getBase64FromInput` 函数中添加白名单检查：

```go
func getBase64FromInput(imageURL string) (string, string, error) {
    // 白名单检查
    allowedHosts := []string{
        "oa.company.com",
        "internal.company.com",
    }
    
    u, err := url.Parse(imageURL)
    if err != nil {
        return "", "", fmt.Errorf("无效的 URL: %w", err)
    }
    
    allowed := false
    for _, host := range allowedHosts {
        if u.Host == host {
            allowed = true
            break
        }
    }
    
    if !allowed {
        return "", "", fmt.Errorf("URL 主机不在白名单中: %s", u.Host)
    }
    
    // 继续处理...
}
```

### 2. HTTPS 支持
- 建议使用 HTTPS URL
- 避免中间人攻击

### 3. 认证
- 如果图片 URL 需要认证，需要在请求头中添加认证信息

## 日志示例

```
2025/10/24 10:30:15 收到Qwen分析请求 - IP: 192.168.1.100
2025/10/24 10:30:15 开始分析请求 - Provider: qwen, UserId: 12345, Alias: 张三, Type: 病假
2025/10/24 10:30:15 开始AI分析 - Provider: qwen, EmployeeName: 张三, 图片来源: URL下载 (http://oa.company.com/upload-file/e025bbcd37180d1b75d95de2e533303a.png)
2025/10/24 10:30:15 Qwen开始处理图片 - 来源: URL下载, URL: http://oa.company.com/upload-file/e025bbcd37180d1b75d95de2e533303a.png, 姓名: 张三, 类型: 病假
2025/10/24 10:30:15 Processing image from URL: http://oa.company.com/upload-file/e025bbcd37180d1b75d95de2e533303a.png
2025/10/24 10:30:15 图片原始格式: png, 原始尺寸: 1080x2340
2025/10/24 10:30:15 图片已缩放至: 461x1000
2025/10/24 10:30:15 图片已重编码为 JPEG (质量: 80), 压缩后大小: 125.43 KB
2025/10/24 10:30:15 图片处理完成 (耗时: 89.234ms, Base64大小: 171234 chars)
2025/10/24 10:30:16 Qwen处理完成 - 总耗时: 1.234s
```

## 总结

这个方案完全可行，具有以下优势：

✅ **透明性** - OA 系统无需关心图片处理细节
✅ **高效性** - 内网下载速度快，处理迅速
✅ **灵活性** - 支持文件上传和 URL 两种方式
✅ **可靠性** - 统一的图片处理逻辑，质量可控
✅ **可扩展性** - 易于添加白名单、认证等功能
