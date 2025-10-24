# 多图片上传支持文档

## 更新概述

系统已更新以支持以下功能：

1. **多图片上传** - 一次可以上传多张图片
2. **智能验证** - 只要有一张图片满足条件即可通过验证
3. **移除 time_validation** - 完全依赖 AI 判断，不再返回 time_validation 字段

## 主要变更

### 1. 支持多图片上传

#### 上传方式

支持三种字段名（按优先级）：
1. `images[]` - 推荐用于多图片上传
2. `images` - 兼容方式
3. `image` - 向后兼容单图片上传

#### 示例代码

**方式一：使用 images[] （推荐）**
```bash
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "application_date=2025-10-24" \
  -F "images[]=@image1.png" \
  -F "images[]=@image2.jpg" \
  -F "images[]=@image3.png"
```

**方式二：使用 images**
```bash
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "application_date=2025-10-24" \
  -F "images=@image1.png" \
  -F "images=@image2.jpg"
```

**方式三：单图片上传（向后兼容）**
```bash
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "application_date=2025-10-24" \
  -F "image=@image1.png"
```

### 2. JavaScript/FormData 示例

```javascript
const formData = new FormData();
formData.append('user_id', '12345');
formData.append('alias', '张三');
formData.append('application_type', '病假');
formData.append('application_date', '2025-10-24');

// 添加多张图片
const files = document.getElementById('fileInput').files;
for (let i = 0; i < files.length; i++) {
  formData.append('images[]', files[i]);
}

// 或者使用 image_url
// formData.append('image_url', 'http://oa.company.com/upload-file/xxx.png');

fetch('http://localhost:8080/api/v1/analyze-qwen', {
  method: 'POST',
  body: formData
})
.then(response => response.json())
.then(data => console.log(data))
.catch(error => console.error('Error:', error));
```

### 3. 混合使用文件和 URL

```bash
# 可以同时上传文件和使用 URL
# 注意：文件和 URL 不能同时提供，会报错

# 正确 - 只使用文件
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "images[]=@image1.png" \
  -F "images[]=@image2.jpg"

# 正确 - 只使用 URL
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "image_url=http://oa.company.com/upload-file/xxx.png"

# 错误 - 不能同时使用
curl -X POST http://localhost:8080/api/v1/analyze-qwen \
  -F "user_id=12345" \
  -F "alias=张三" \
  -F "application_type=病假" \
  -F "images[]=@image1.png" \
  -F "image_url=http://oa.company.com/upload-file/xxx.png"
# 错误响应: {"error":"请求无效","details":"图片文件和图片URL不能同时上传"}
```

## 处理逻辑

### 图片验证流程

```
1. 接收多张图片
   ↓
2. 逐张调用 AI 分析
   ↓
3. 检查每张图片的 IsProofTypeValid
   ↓
4. 找到第一张满足条件的图片
   ↓
5. 停止处理后续图片
   ↓
6. 使用该图片的提取数据进行规则验证
```

### 关键特性

#### 短路逻辑
- 一旦找到有效图片，立即停止处理
- 节省 AI 调用成本和处理时间

#### 错误处理
- 单张图片分析失败不影响其他图片
- 记录所有错误信息
- 只有所有图片都失败才返回错误

#### 日志输出
```log
收到 3 张图片（images[]）
开始AI分析 - Provider: qwen, EmployeeName: 张三, 总图片数: 3
分析第 1/3 张图片（文件上传，大小: 2048576 bytes）
第 1 张图片分析完成 (耗时: 1.234s): IsProofTypeValid=false, ExtractedName=张三
分析第 2/3 张图片（文件上传，大小: 1536789 bytes）
第 2 张图片分析完成 (耗时: 1.156s): IsProofTypeValid=true, ExtractedName=张三
✓ 第 2 张图片满足条件，停止处理后续图片
```

## 响应格式变更

### 新的响应格式（移除了 time_validation）

```json
{
  "is_abnormal": false,
  "reason": "正常",
  "raw_text": ""
}
```

### 旧的响应格式（已废弃）

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
  "raw_text": ""
}
```

**注意**：`time_validation` 字段已完全移除，验证逻辑完全由 AI 判断。

## 各种场景的响应示例

### 场景 1: 多张图片，第一张满足条件

**请求**：
- 上传 3 张图片
- 第 1 张：病历单（有效）
- 第 2 张：处方单（未处理）
- 第 3 张：诊断证明（未处理）

**响应**：
```json
{
  "is_abnormal": false,
  "reason": "正常"
}
```

**日志**：
```
✓ 第 1 张图片满足条件，停止处理后续图片
总分析时间: 1.5s
```

### 场景 2: 多张图片，第二张满足条件

**请求**：
- 上传 3 张图片
- 第 1 张：无关图片（无效）
- 第 2 张：病历单（有效）
- 第 3 张：诊断证明（未处理）

**响应**：
```json
{
  "is_abnormal": false,
  "reason": "正常"
}
```

**日志**：
```
第 1 张图片分析完成: IsProofTypeValid=false
第 2 张图片分析完成: IsProofTypeValid=true
✓ 第 2 张图片满足条件，停止处理后续图片
总分析时间: 2.8s
```

### 场景 3: 所有图片都不满足条件

**请求**：
- 上传 3 张图片
- 第 1 张：无关图片（无效）
- 第 2 张：无关图片（无效）
- 第 3 张：无关图片（无效）

**响应**：
```json
{
  "is_abnormal": true,
  "reason": "所有提供的图片均不是有效的证明材料"
}
```

**日志**：
```
第 1 张图片分析完成: IsProofTypeValid=false
第 2 张图片分析完成: IsProofTypeValid=false
第 3 张图片分析完成: IsProofTypeValid=false
所有图片均不满足条件
```

### 场景 4: 部分图片分析失败

**请求**：
- 上传 3 张图片
- 第 1 张：图片损坏（分析失败）
- 第 2 张：病历单（有效）
- 第 3 张：诊断证明（未处理）

**响应**：
```json
{
  "is_abnormal": false,
  "reason": "正常"
}
```

**日志**：
```
第 1 张图片分析失败: 无法解码图片
第 2 张图片分析完成: IsProofTypeValid=true
✓ 第 2 张图片满足条件，停止处理后续图片
```

### 场景 5: 所有图片分析都失败

**请求**：
- 上传 2 张图片
- 第 1 张：图片损坏（分析失败）
- 第 2 张：格式不支持（分析失败）

**响应**：
```json
{
  "error": "Qwen 分析失败",
  "details": "所有图片分析均失败: 第 1 张图片分析失败: 无法解码图片"
}
```

**HTTP 状态码**：500

## 性能影响

### 处理时间估算

| 场景 | 图片数量 | 有效图片位置 | 预计时间 | 说明 |
|------|---------|-------------|---------|------|
| 最佳 | 3 | 第1张 | ~1.5s | 立即找到有效图片 |
| 一般 | 3 | 第2张 | ~3s | 需要处理2张图片 |
| 最差 | 3 | 第3张或都无效 | ~4.5s | 处理所有图片 |

### 优化建议

1. **图片顺序**：建议将最可能有效的图片放在前面
2. **图片数量**：建议不超过 5 张，避免过长的处理时间
3. **图片大小**：建议每张图片 < 5MB，加快上传和处理速度

## 兼容性说明

### ✅ 向后兼容

- 单图片上传方式完全兼容
- 使用 `image` 字段的旧代码无需修改
- API 响应格式简化，移除了 `time_validation` 字段

### ⚠️ 破坏性变更

- **time_validation 字段已移除**
  - 如果您的前端代码依赖此字段，需要移除相关逻辑
  - 所有验证现在完全由 AI 判断，通过 `is_abnormal` 和 `reason` 字段返回

### 迁移指南

**旧代码**：
```javascript
if (response.time_validation && response.time_validation.risk_level === 'high') {
  showWarning('时间验证风险高');
}
```

**新代码**：
```javascript
if (response.is_abnormal) {
  showWarning(response.reason);
}
```

## 常见问题

### Q1: 为什么要移除 time_validation？

**A**: 
1. AI 已经具备时间判断能力，不需要额外的时间验证
2. 简化响应结构，减少混淆
3. time_validation 的返回内容经常不准确

### Q2: 如何知道使用了哪张图片？

**A**: 查看日志输出，会显示 "✓ 第 X 张图片满足条件"

### Q3: 可以上传多少张图片？

**A**: 
- 技术上无限制
- 建议不超过 5 张
- 处理时间随图片数量线性增长

### Q4: 如果所有图片都不满足怎么办？

**A**: 系统会返回 `is_abnormal: true` 和原因说明

### Q5: 支持哪些图片格式？

**A**: JPEG, PNG, GIF, BMP, WebP（会自动转换为 JPEG 处理）

## 测试建议

### 1. 单元测试场景

```bash
# 测试 1: 单张有效图片
# 测试 2: 多张图片，第一张有效
# 测试 3: 多张图片，中间一张有效
# 测试 4: 多张图片，最后一张有效
# 测试 5: 所有图片都无效
# 测试 6: 部分图片损坏
# 测试 7: 文件和 URL 同时提供（应该失败）
# 测试 8: 只使用 URL
```

### 2. 压力测试

```bash
# 并发 10 个请求，每个请求 3 张图片
for i in {1..10}; do
  curl -X POST http://localhost:8080/api/v1/analyze-qwen \
    -F "user_id=test_$i" \
    -F "alias=测试用户$i" \
    -F "application_type=病假" \
    -F "images[]=@image1.png" \
    -F "images[]=@image2.jpg" \
    -F "images[]=@image3.png" &
done
wait
```

## 总结

### ✅ 新功能

- [x] 支持多图片上传（最多推荐 5 张）
- [x] 智能短路逻辑（找到有效图片立即停止）
- [x] 移除 time_validation（完全依赖 AI）
- [x] 向后兼容单图片上传
- [x] 支持文件上传和 URL 下载

### 📈 优势

1. **提高通过率** - 多张图片增加验证成功的概率
2. **节省成本** - 找到有效图片立即停止，减少 AI 调用
3. **用户友好** - 用户可以一次上传多张证明材料
4. **简化响应** - 移除不准确的 time_validation 字段

### 🎯 使用建议

1. 前端建议支持拖拽上传多张图片
2. 按照可能性排序图片（最可能有效的放前面）
3. 更新前端代码移除 time_validation 依赖
4. 添加图片预览功能，方便用户检查
