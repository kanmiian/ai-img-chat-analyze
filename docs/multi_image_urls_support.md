# 多图片 URL 支持和详细分析结果

## 更新内容

本次更新解决了以下问题：

1. ✅ **支持多个 image_url** - 可以同时传递多个图片 URL
2. ✅ **返回详细分析结果** - 包括每张图片的提取内容和判断结果
3. ✅ **完整的错误信息** - 显示每张图片的失败原因

## 使用方法

### 方式一：使用 image_urls[] 字段（推荐）

```bash
curl --location --request POST 'localhost:8080/api/v1/analyze-volcano' \
--form 'application_type="补卡"' \
--form 'application_time="09:05"' \
--form 'application_date="2025-08-05"' \
--form 'alias="甘美欣"' \
--form 'image_urls[]="https://oa.example.com/file1.png"' \
--form 'image_urls[]="https://oa.example.com/file2.jpg"' \
--form 'image_urls[]="https://oa.example.com/file3.png"'
```

### 方式二：多次使用 image_url 字段

**注意**：这种方式在某些情况下只会保留最后一个值，建议使用 `image_urls[]` 字段。

```bash
curl --location --request POST 'localhost:8080/api/v1/analyze-volcano' \
--form 'application_type="补卡"' \
--form 'alias="甘美欣"' \
--form 'image_url="https://oa.example.com/file1.png"'
```

### 方式三：混合使用（向后兼容）

系统会自动合并 `image_url` 和 `image_urls[]`：

```bash
curl --location --request POST 'localhost:8080/api/v1/analyze-volcano' \
--form 'application_type="补卡"' \
--form 'alias="甘美欣"' \
--form 'image_url="https://oa.example.com/file1.png"' \
--form 'image_urls[]="https://oa.example.com/file2.jpg"'
```

## 响应格式

### 成功找到有效图片

```json
{
  "is_abnormal": false,
  "reason": "正常",
  "valid_image_index": 2,
  "images_analysis": [
    {
      "index": 1,
      "source": "url_download",
      "image_url": "https://oa.example.com/file1.png",
      "success": true,
      "extracted_data": {
        "extracted_name": "甘美欣",
        "request_date": "2025-08-05",
        "request_time": "09:05",
        "request_type": "浏览器记录",
        "is_proof_type_valid": false,
        "content": "显示时间为 09:05，但不是有效的补卡证明"
      },
      "processing_time_ms": 1234,
      "is_valid": false
    },
    {
      "index": 2,
      "source": "url_download",
      "image_url": "https://oa.example.com/file2.jpg",
      "success": true,
      "extracted_data": {
        "extracted_name": "甘美欣",
        "request_date": "2025-08-05",
        "request_time": "09:05",
        "request_type": "系统截图",
        "is_proof_type_valid": true,
        "content": "钉钉打卡记录截图，显示时间 09:05"
      },
      "processing_time_ms": 1456,
      "is_valid": true
    }
  ]
}
```

### 所有图片都不满足条件

```json
{
  "is_abnormal": true,
  "reason": "所有提供的图片均不是有效的证明材料",
  "valid_image_index": 0,
  "images_analysis": [
    {
      "index": 1,
      "source": "url_download",
      "image_url": "https://oa.example.com/file1.png",
      "success": true,
      "extracted_data": {
        "extracted_name": "甘美欣",
        "request_date": "2025-08-05",
        "request_time": "未知",
        "request_type": "未知",
        "is_proof_type_valid": false,
        "content": "无法识别有效内容"
      },
      "processing_time_ms": 1234,
      "is_valid": false
    },
    {
      "index": 2,
      "source": "url_download",
      "image_url": "https://oa.example.com/file2.jpg",
      "success": true,
      "extracted_data": {
        "extracted_name": "张三",
        "request_date": "2025-08-05",
        "request_time": "未知",
        "request_type": "病历单",
        "is_proof_type_valid": false,
        "content": "这是病历单，不适用于补卡申请"
      },
      "processing_time_ms": 1456,
      "is_valid": false
    }
  ]
}
```

### 部分图片分析失败

```json
{
  "is_abnormal": false,
  "reason": "正常",
  "valid_image_index": 3,
  "images_analysis": [
    {
      "index": 1,
      "source": "url_download",
      "image_url": "https://oa.example.com/file1.png",
      "success": false,
      "error_message": "无法下载图片 URL: Get \"https://oa.example.com/file1.png\": context deadline exceeded",
      "processing_time_ms": 10000,
      "is_valid": false
    },
    {
      "index": 2,
      "source": "url_download",
      "image_url": "https://oa.example.com/file2.jpg",
      "success": false,
      "error_message": "无法解码图片: image: unknown format",
      "processing_time_ms": 234,
      "is_valid": false
    },
    {
      "index": 3,
      "source": "url_download",
      "image_url": "https://oa.example.com/file3.png",
      "success": true,
      "extracted_data": {
        "extracted_name": "甘美欣",
        "request_date": "2025-08-05",
        "request_time": "09:05",
        "request_type": "系统截图",
        "is_proof_type_valid": true,
        "content": "有效的补卡证明"
      },
      "processing_time_ms": 1456,
      "is_valid": true
    }
  ]
}
```

## 响应字段说明

### 根级别字段

| 字段名 | 类型 | 说明 |
|--------|------|------|
| is_abnormal | boolean | 是否异常 |
| reason | string | 判断原因 |
| valid_image_index | int | 有效图片的索引（从1开始，0表示无有效图片） |
| images_analysis | array | 所有图片的详细分析结果 |

### images_analysis 数组元素

| 字段名 | 类型 | 说明 |
|--------|------|------|
| index | int | 图片索引（从1开始） |
| source | string | 图片来源：file_upload 或 url_download |
| file_name | string | 文件名（仅文件上传时） |
| image_url | string | 图片URL（仅URL下载时） |
| success | boolean | 是否分析成功 |
| error_message | string | 错误信息（仅失败时） |
| extracted_data | object | 提取的数据（仅成功时） |
| processing_time_ms | int | 处理时间（毫秒） |
| is_valid | boolean | 是否为有效证明材料 |

### extracted_data 对象

| 字段名 | 类型 | 说明 |
|--------|------|------|
| extracted_name | string | 提取的姓名 |
| request_date | string | 提取的日期（yyyy-MM-dd） |
| request_time | string | 提取的时间（HH:mm） |
| request_type | string | 识别的图片类型 |
| is_proof_type_valid | boolean | 是否为有效的证明材料类型 |
| content | string | 提取的关键文字内容 |

## 日志输出示例

```log
2025-10-24 10:30:15 收到Volcano分析请求 - IP: 192.168.1.100
2025-10-24 10:30:15 解析完成 - 文件数: 0, URL数: 2
2025-10-24 10:30:15 开始分析请求 - Provider: volcano, UserId: , Alias: 甘美欣, Type: 补卡, 图片数量: 0
2025-10-24 10:30:15 开始AI分析 - Provider: volcano, EmployeeName: 甘美欣, 总图片数: 2 (文件: 0, URL: 2)

2025-10-24 10:30:15 分析第 1/2 张图片（URL下载: https://oa.example.com/file1.png）
2025-10-24 10:30:15 Processing image from URL: https://oa.example.com/file1.png
2025-10-24 10:30:16 图片原始格式: png, 原始尺寸: 1080x2340
2025-10-24 10:30:16 图片已缩放至: 461x1000
2025-10-24 10:30:16 图片已重编码为 JPEG (质量: 80), 压缩后大小: 125.43 KB
2025-10-24 10:30:16 图片处理完成 (耗时: 1.234s, Base64大小: 171234 chars)
2025-10-24 10:30:17 第 1 张图片分析完成 (耗时: 2.456s): IsProofTypeValid=false, ExtractedName=甘美欣, RequestType=浏览器记录, Content=显示时间为 09:05，但不是有效的补卡证明

2025-10-24 10:30:17 分析第 2/2 张图片（URL下载: https://oa.example.com/file2.jpg）
2025-10-24 10:30:17 Processing image from URL: https://oa.example.com/file2.jpg
2025-10-24 10:30:18 图片原始格式: jpeg, 原始尺寸: 800x1200
2025-10-24 10:30:18 图片处理完成 (耗时: 0.890s, Base64大小: 98765 chars)
2025-10-24 10:30:19 第 2 张图片分析完成 (耗时: 1.890s): IsProofTypeValid=true, ExtractedName=甘美欣, RequestType=系统截图, Content=钉钉打卡记录截图，显示时间 09:05
2025-10-24 10:30:19 ✓ 第 2 张图片满足条件，停止处理后续图片

2025-10-24 10:30:19 开始规则引擎验证
2025-10-24 10:30:19 规则引擎验证完成 (耗时: 15ms)
2025-10-24 10:30:19 总分析时间: 4.5s, 结果: IsAbnormal=false, Reason=正常, 有效图片索引=2
2025-10-24 10:30:19 Volcano分析完成 (总耗时: 4.5s) - 结果: IsAbnormal=false
```

## 前端处理建议

### 解析响应并显示详情

```javascript
fetch('http://localhost:8080/api/v1/analyze-volcano', {
  method: 'POST',
  body: formData
})
.then(response => response.json())
.then(data => {
  // 显示最终结果
  if (data.is_abnormal) {
    console.error('分析异常:', data.reason);
  } else {
    console.log('分析正常:', data.reason);
  }
  
  // 显示有效图片
  if (data.valid_image_index > 0) {
    console.log('有效图片索引:', data.valid_image_index);
  }
  
  // 显示所有图片的详细分析
  data.images_analysis.forEach((img, index) => {
    console.log(`\n图片 ${img.index}:`);
    console.log('  来源:', img.source);
    console.log('  成功:', img.success);
    
    if (!img.success) {
      console.error('  错误:', img.error_message);
    } else {
      console.log('  有效:', img.is_valid);
      console.log('  提取的姓名:', img.extracted_data.extracted_name);
      console.log('  识别类型:', img.extracted_data.request_type);
      console.log('  内容:', img.extracted_data.content);
    }
    
    console.log('  处理时间:', img.processing_time_ms, 'ms');
  });
})
.catch(error => console.error('Error:', error));
```

### 表格展示

```javascript
function renderAnalysisTable(imagesAnalysis) {
  const table = document.createElement('table');
  table.innerHTML = `
    <thead>
      <tr>
        <th>序号</th>
        <th>来源</th>
        <th>状态</th>
        <th>有效</th>
        <th>提取姓名</th>
        <th>类型</th>
        <th>内容</th>
        <th>耗时</th>
      </tr>
    </thead>
    <tbody>
      ${imagesAnalysis.map(img => `
        <tr class="${img.is_valid ? 'valid' : 'invalid'}">
          <td>${img.index}</td>
          <td>${img.source === 'file_upload' ? '文件上传' : 'URL下载'}</td>
          <td>${img.success ? '✓ 成功' : '✗ 失败'}</td>
          <td>${img.is_valid ? '✓ 有效' : '✗ 无效'}</td>
          <td>${img.extracted_data?.extracted_name || '-'}</td>
          <td>${img.extracted_data?.request_type || img.error_message || '-'}</td>
          <td>${img.extracted_data?.content || '-'}</td>
          <td>${img.processing_time_ms}ms</td>
        </tr>
      `).join('')}
    </tbody>
  `;
  return table;
}
```

## 数据模型变更

### ApplicationData

```go
type ApplicationData struct {
    UserId          string   `form:"user_id"`
    Alias           string   `form:"alias"`
    ApplicationType string   `form:"application_type"`
    ApplicationTime string   `form:"application_time"`
    ApplicationDate string   `form:"application_date"`
    Reason          string   `form:"reason"`
    ImageUrl        string   `form:"image_url"`        // 单个 URL（向后兼容）
    ImageUrls       []string `form:"image_urls[]"`     // 多个 URLs（新增）
}
```

### AnalysisResult

```go
type AnalysisResult struct {
    IsAbnormal       bool                   `json:"is_abnormal"`
    Reason           string                 `json:"reason"`
    ValidImageIndex  int                    `json:"valid_image_index,omitempty"`  // 新增
    ImagesAnalysis   []ImageAnalysisDetail  `json:"images_analysis,omitempty"`    // 新增
    TimeValidation   *TimeValidationResult  `json:"time_validation,omitempty"`
    RawText          string                 `json:"raw_text,omitempty"`
}
```

### ImageAnalysisDetail（新增）

```go
type ImageAnalysisDetail struct {
    Index            int            `json:"index"`
    Source           string         `json:"source"`
    FileName         string         `json:"file_name,omitempty"`
    ImageURL         string         `json:"image_url,omitempty"`
    Success          bool           `json:"success"`
    ErrorMessage     string         `json:"error_message,omitempty"`
    ExtractedData    *ExtractedData `json:"extracted_data,omitempty"`
    ProcessingTimeMs int64          `json:"processing_time_ms"`
    IsValid          bool           `json:"is_valid"`
}
```

## 常见问题

### Q1: 为什么我传了多个 image_url 但只识别了一个？

**A**: 因为 HTTP 表单中相同的字段名会被覆盖。建议使用 `image_urls[]` 字段：

```bash
# ❌ 错误方式 - 只会保留最后一个
--form 'image_url="url1"' \
--form 'image_url="url2"'

# ✅ 正确方式
--form 'image_urls[]="url1"' \
--form 'image_urls[]="url2"'
```

### Q2: 如何判断哪张图片是有效的？

**A**: 查看响应中的 `valid_image_index` 字段，它表示有效图片的索引（从1开始）。

### Q3: images_analysis 数组的顺序是什么？

**A**: 按照处理顺序排列。如果找到有效图片后停止处理，后续图片不会出现在数组中。

### Q4: 如何获取图片的详细提取内容？

**A**: 查看 `images_analysis` 数组中每个元素的 `extracted_data` 字段。

### Q5: 文件上传和 URL 可以混用吗？

**A**: 不可以。系统会返回错误："图片文件和图片URL不能同时上传"。

## 性能影响

### 处理时间

- 每个 URL 下载：50-200ms（取决于网络）
- 每张图片处理：800-1500ms（取决于图片大小）
- 总处理时间 = 下载时间 + 处理时间 + AI分析时间

### 建议

1. **图片数量**：建议不超过 5 张
2. **URL 可访问性**：确保 Go 服务能访问图片 URL
3. **图片大小**：建议每张 < 5MB
4. **超时设置**：URL 下载超时为 10 秒

## 总结

### ✅ 新功能

- [x] 支持多个图片 URL（使用 `image_urls[]` 字段）
- [x] 向后兼容单个 `image_url` 字段
- [x] 返回每张图片的详细分析结果
- [x] 返回每张图片的提取内容
- [x] 返回每张图片的错误信息
- [x] 标记哪张图片是有效的

### 📊 数据透明度

现在您可以：
- 查看每张图片的分析结果
- 了解为什么某张图片无效
- 知道 AI 提取了什么内容
- 追踪每张图片的处理时间

### 🎯 使用建议

1. 使用 `image_urls[]` 字段传递多个 URL
2. 前端展示详细的分析结果给用户
3. 记录和监控图片处理成功率
4. 根据 `extracted_data` 验证 AI 判断的准确性
