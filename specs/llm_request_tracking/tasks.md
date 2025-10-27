# 实施计划 - LLM请求追踪与图片URL优化

## 任务列表

### 1. 修改数据模型层
- 在 `client/llm_share.go` 中的 `LlmResponse` 结构体添加 `Id` 字段
- 在 `model/models.go` 中的 `ImageAnalysisDetail` 结构体添加 `RequestId` 字段
- _需求: 需求1_

### 2. 重构图片处理逻辑（支持URL直传）
- 在 `client/llm_share.go` 中新增 `buildImageContentPart` 函数
- 该函数根据输入类型（文件或URL）返回不同的 `ContentPart`
- 文件上传：调用 `imageToBase64` 生成 data URI
- URL方式：直接使用HTTP URL
- _需求: 需求2_

### 3. 修改 Qwen 客户端
- 修改 `client/qwen_client.go` 中的 `ExtractDataFromImage` 方法签名
- 返回值改为三元组：`(*model.ExtractedData, string, error)`
- 使用 `buildImageContentPart` 替代原有的 `processImageInput`
- 从 `LlmResponse` 中提取 `Id` 字段并返回
- 更新日志输出，记录处理方式和requestId
- _需求: 需求1, 需求2_

### 4. 修改 Volcano 客户端
- 修改 `client/volcano_client.go` 中的 `ExtractDataFromImage` 方法签名
- 返回值改为三元组：`(*model.ExtractedData, string, error)`
- 使用 `buildImageContentPart` 替代原有的 `processImageInput`
- 从 `LlmResponse` 中提取 `Id` 字段并返回
- 更新日志输出，记录处理方式和requestId
- _需求: 需求1, 需求2_

### 5. 修改 Service 层
- 修改 `service/analysis_service.go` 中的 `runAnalysis` 方法
- 更新所有调用 `ExtractDataFromImage` 的地方，接收返回的 requestId
- 在构建 `ImageAnalysisDetail` 时设置 `RequestId` 字段
- _需求: 需求1_

### 6. 清理废弃代码（可选）
- 评估 `processImageInput` 和 `getBase64FromInput` 函数是否还被使用
- 如果仅用于图片处理，可以移除 `getBase64FromInput` 函数（因为URL不再需要下载）
- 保留 `imageToBase64` 函数（文件上传仍需要）
- _需求: 需求2_

### 7. 测试验证
- 测试文件上传方式，验证base64转换和requestId返回
- 测试单个URL方式，验证直接传递和requestId返回
- 测试多个URL方式，验证每个URL都有对应的requestId
- 测试混合场景（文件+URL），验证处理正确性
- 验证错误场景（无效URL），检查错误信息记录
- _需求: 需求1, 需求2, 需求3_

### 8. 日志和文档更新
- 更新相关API文档，说明新增的 `request_id` 字段
- 更新CHANGELOG，记录本次优化
- _需求: 需求3_

## 预估工作量

- **开发时间**：2-3小时
- **测试时间**：1小时
- **总计**：3-4小时

## 风险评估

- **低风险**：改动范围明确，主要是添加字段和优化逻辑
- **兼容性**：新增字段使用 `omitempty`，不破坏现有API
- **回滚方案**：可快速回退到旧版本

