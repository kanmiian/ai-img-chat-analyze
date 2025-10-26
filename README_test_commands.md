# 测试命令说明

## 接口支持的功能

### 1. 多图片 URL 支持 ✅
接口已支持传递多个图片 URL，方式：
- 使用 `image_urls[]` 字段（推荐）
- 混合使用 `image_url` 和 `image_urls[]`

### 2. 直接使用图片 URL（不进行 Base64 转换）✅ 新增
新增功能：可以通过 `use_direct_url=true` 参数，直接传递图片 URL 给 AI，跳过 Base64 转换过程。

#### 优势
- **性能提升**：跳过下载、解码、重编码、压缩等步骤
- **减少内存占用**：不需要将图片加载到内存
- **更快的响应时间**：适合 AI 服务能直接访问图片 URL 的场景

#### 使用场景
1. AI 服务可以访问图片 URL（同内网、有公网访问权限）
2. 图片已经托管在可访问的服务器上
3. 需要提高处理速度，减少服务器负载

---

## 快速测试命令

### 测试 1：多图片 URL（使用 Base64 转换）
```bash
curl.exe -X POST http://localhost:8080/api/v1/analyze-volcano ^
  -F "user_id=12345" ^
  -F "alias=张三" ^
  -F "application_type=病假" ^
  -F "application_date=2025-01-24" ^
  -F "application_time=18:32" ^
  -F "reason=生病请假" ^
  -F "image_urls[]=https://oa.shiyuegame.com/aetherupload/display/file/20250124/e1cf7a84900ad106b2ddd936abced53e.png/" ^
  -F "image_urls[]=https://oa.shiyuegame.com/aetherupload/display/file/20250124/02bd29b81fdcd335958dcca17a608a1f.jpg/"
```

### 测试 2：多图片 URL（直接使用 URL，不转换）
```bash
curl.exe -X POST http://localhost:8080/api/v1/analyze-volcano ^
  -F "user_id=12345" ^
  -F "alias=张三" ^
  -F "application_type=病假" ^
  -F "application_date=2025-01-24" ^
  -F "application_time=18:32" ^
  -F "reason=生病请假" ^
  -F "image_urls[]=https://oa.shiyuegame.com/aetherupload/display/file/20250124/e1cf7a84900ad106b2ddd936abced53e.png/" ^
  -F "image_urls[]=https://oa.shiyuegame.com/aetherupload/display/file/20250124/02bd29b81fdcd335958dcca17a608a1f.jpg/" ^
  -F "use_direct_url=true"
```

### 测试 3：单张图片，直接使用 URL
```bash
curl.exe -X POST http://localhost:8080/api/v1/analyze-volcano ^
  -F "user_id=12345" ^
  -F "alias=张三" ^
  -F "application_type=病假" ^
  -F "application_date=2025-01-24" ^
  -F "application_time=18:43" ^
  -F "reason=生病请假" ^
  -F "image_url=https://oa.shiyuegame.com/aetherupload/display/file/20250124/e1cf7a84900ad106b2ddd936abced53e.png/" ^
  -F "use_direct_url=true"
```

---

## 参数说明

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| user_id | string | 是 | 员工 ID |
| alias | string | 是 | 员工姓名 |
| application_type | string | 是 | 申请类型（病假/补打卡/事假/年假） |
| application_date | string | 可选 | 申请日期（YYYY-MM-DD） |
| application_time | string | 可选 | 申请时间（HH:mm） |
| reason | string | 可选 | 申请理由 |
| image_url | string | 可选* | 单个图片 URL |
| image_urls[] | array | 可选* | 多个图片 URL |
| use_direct_url | string | 可选 | 是否直接使用 URL（true/false） |

*注：`image_url` 和 `image_urls[]` 至少传一个（当申请类型为病假或补打卡时必填）

---

## 响应格式

### 成功响应
```json
{
  "is_abnormal": false,
  "reason": "正常",
  "valid_image_index": 1,
  "images_analysis": [
    {
      "index": 1,
      "source": "url_download",
      "image_url": "https://oa.shiyuegame.com/aetherupload/display/file/...",
      "success": true,
      "extracted_data": {
        "extracted_name": "张三",
        "request_date": "2025-01-24",
        "request_time": "18:32",
        "request_type": "病历单",
        "is_proof_type_valid": true,
        "content": "病历单内容..."
      },
      "processing_time_ms": 1234,
      "is_valid": true
    }
  ]
}
```

---

## 详细测试文件

- `test_image_url.txt` - 单张图片测试命令
- `test_multi_images.txt` - 多张图片测试命令
- `test_direct_url.txt` - 直接使用 URL 测试命令（新增功能）

---

## 注意事项

1. **use_direct_url 参数**
   - 当 `use_direct_url=true` 时，AI 服务必须能够访问图片 URL
   - 如果 AI 服务无法访问图片 URL，请求会失败
   - 默认值为 `false`（进行 Base64 转换），保持向后兼容

2. **多图片处理**
   - 支持传递多个图片 URL
   - 只要有一张图片满足条件，就会停止处理后续图片
   - 所有图片的分析结果都会在 `images_analysis` 数组中返回

3. **性能对比**
   - Base64 转换模式：稳定、兼容性好，但处理时间较长
   - 直接 URL 模式：处理速度快，但需要 AI 服务能访问 URL
