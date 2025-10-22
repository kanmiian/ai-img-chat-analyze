# My AI App

基于 Go 的 AI 图片分析应用，集成 PaddleOCR 和火山引擎。

## 功能特性

- ✅ OCR 文字识别（PaddleOCR）
- ✅ 火山引擎集成
- ✅ 规则引擎
- ✅ RESTful API
- ✅ Docker 容器化部署

## 项目结构

```
my-ai-app/
├── api/                    # API 处理器
│   └── upload_handler.go
├── client/                 # 客户端
│   ├── ocr_client.go      # PaddleOCR 客户端
│   └── volcano_client.go  # 火山引擎客户端
├── config/                 # 配置管理
│   └── config.go
├── model/                  # 数据模型
│   └── models.go
├── rules/                  # 规则引擎
│   └── engine.go
├── service/                # 业务逻辑
│   └── analysis_service.go
├── .env.example           # 环境变量示例
├── Dockerfile             # Docker 构建文件
├── docker-compose.yml     # Docker 编排文件
├── go.mod                 # Go 模块
└── main.go               # 程序入口
```

## 快速开始

### 前置要求

- Go 1.21+
- Docker & Docker Compose

### 本地开发

1. 克隆项目
```bash
git clone <repository>
cd my-ai-app
```

2. 安装依赖
```bash
go mod download
```

3. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env 文件，填入火山引擎的 AccessKey 和 SecretKey
```

4. 运行应用
```bash
go run main.go
```

### Docker 部署

1. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env 文件
```

2. 启动服务
```bash
docker-compose up -d
```

3. 查看日志
```bash
docker-compose logs -f
```

## API 文档

### 健康检查

```
GET /health
```

### 上传图片分析

```
POST /api/v1/upload
Content-Type: application/json

{
  "image": "base64_encoded_image",
  "options": {
    "use_volcano": "true"
  },
  "metadata": {
    "source": "mobile_app"
  }
}
```

响应示例：
```json
{
  "success": true,
  "message": "分析成功",
  "data": {
    "ocr": {
      "text": "识别的文本内容",
      "confidence": 0.95,
      "boxes": [...]
    },
    "rule_matches": [...],
    "summary": "识别到 5 个文本块，平均置信度 0.95。",
    "process_time": 1234
  }
}
```

### URL 图片分析

```
POST /api/v1/analyze
Content-Type: application/json

{
  "image_url": "https://example.com/image.jpg",
  "type": "document",
  "options": {}
}
```

## 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| SERVER_PORT | 服务器端口 | 8080 |
| SERVER_MODE | 运行模式 (debug/release) | debug |
| OCR_SERVICE_URL | PaddleOCR 服务地址 | http://paddleocr:8868 |
| OCR_TIMEOUT | OCR 超时时间（秒） | 30 |
| VOLCANO_ACCESS_KEY | 火山引擎 AccessKey | - |
| VOLCANO_SECRET_KEY | 火山引擎 SecretKey | - |
| VOLCANO_REGION | 火山引擎区域 | cn-beijing |

## 开发指南

### 添加新规则

在 `rules/engine.go` 中实现 `Rule` 接口：

```go
type CustomRule struct {
    ID   string
    Name string
}

func (r *CustomRule) Match(text string, context map[string]interface{}) model.RuleMatch {
    // 实现匹配逻辑
}
```

### 扩展 API

在 `api/` 目录下创建新的处理器文件，并在 `main.go` 中注册路由。

## 许可证

MIT License

