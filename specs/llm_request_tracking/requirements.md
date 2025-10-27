# 需求文档 - LLM请求追踪与图片URL优化

## 介绍

当前系统在调用LLM API进行图片分析时，存在两个问题：
1. 未返回LLM API的请求ID（requestId），导致无法追踪和关联具体的LLM调用记录
2. 对于已经是URL形式的图片，仍会下载并转换为base64后再发送给LLM，增加了处理时间和资源消耗

本需求旨在优化以上两个问题，提升系统的可追溯性和性能。

## 需求

### 需求 1 - 返回LLM请求ID

**用户故事：** 作为系统管理员，我希望在每次调用结果中能够看到LLM API返回的请求ID，以便追踪问题、审计日志或关联LLM服务商的账单记录。

#### 验收标准

1. When 系统调用LLM API（Qwen或Volcano）完成图片分析时，系统应当从LLM响应中提取requestId字段
2. When API返回分析结果时，系统应当在每个图片分析详情（`ImagesAnalysis`数组中的每个元素）中包含对应的`request_id`字段
3. When LLM API未返回requestId或调用失败时，系统应当将`request_id`字段设置为空字符串或省略该字段
4. While 使用Qwen或Volcano任一provider时，系统都应当正确提取和返回requestId

### 需求 2 - 图片URL直接传递优化

**用户故事：** 作为开发者，当我通过URL方式提供图片时，我希望系统能够直接将URL传递给LLM API，而不是先下载再转base64，以减少处理时间和服务器资源消耗。

#### 验收标准

1. When 用户通过`image_url`或`image_urls[]`字段提供图片URL时，系统应当直接将URL传递给LLM API，而不进行base64转换
2. When 用户通过文件上传方式提供图片时，系统应当继续使用base64编码方式（因为文件无公网可访问URL）
3. When 直接传递URL给LLM API时，系统应当使用符合OpenAI Vision API规范的格式：`{"type": "image_url", "image_url": {"url": "http://..."}}`
4. While 处理URL图片时，系统不应当执行图片下载、解码、缩放、重编码等操作
5. When URL图片处理失败时，系统应当在响应中记录错误信息，但不影响其他图片的处理

### 需求 3 - 向后兼容性

**用户故事：** 作为现有API用户，我希望新的改动不会破坏现有的API接口和数据结构。

#### 验收标准

1. When 系统升级后，现有的API接口路径、请求参数格式应当保持不变
2. When 系统返回响应时，新增的`request_id`字段应当为可选字段，不影响现有客户端的JSON解析
3. When 使用文件上传方式时，系统应当保持原有的处理逻辑（base64转换）

