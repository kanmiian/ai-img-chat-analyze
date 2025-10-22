# 项目初始化指南

## 初始化步骤

### 1. 初始化 Go 模块
```bash
go mod tidy
```

这会生成 `go.sum` 文件并下载所有依赖。

### 2. 配置环境变量
```bash
# Windows
copy env.example .env

# Linux/Mac
cp env.example .env
```

然后编辑 `.env` 文件，填入您的火山引擎配置：
- VOLCANO_ACCESS_KEY
- VOLCANO_SECRET_KEY

### 3. 本地运行（需要先启动 PaddleOCR 服务）

#### 方式一：使用 Docker Compose（推荐）
```bash
docker-compose up -d
```

#### 方式二：本地开发
```bash
# 终端 1：启动 PaddleOCR 服务（需要 Docker）
docker run -d --name paddleocr -p 8868:8868 paddlepaddle/serving:latest-cpu-py38

# 终端 2：启动 Go 应用
go run main.go
```

### 4. 测试 API

#### 健康检查
```bash
curl http://localhost:8080/health
```

#### 上传图片分析
```bash
curl -X POST http://localhost:8080/api/v1/upload \
  -H "Content-Type: application/json" \
  -d '{
    "image": "base64_encoded_image_data",
    "options": {
      "use_volcano": "false"
    }
  }'
```

#### URL 图片分析
```bash
curl -X POST http://localhost:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "image_url": "https://example.com/image.jpg",
    "type": "document"
  }'
```

## 常见问题

### Q: go.sum 文件缺失？
A: 运行 `go mod tidy` 命令生成。

### Q: PaddleOCR 服务连接失败？
A: 检查 `OCR_SERVICE_URL` 环境变量是否正确，确保 PaddleOCR 容器正在运行。

### Q: 火山引擎调用失败？
A: 检查 `VOLCANO_ACCESS_KEY` 和 `VOLCANO_SECRET_KEY` 是否正确配置。

## 下一步开发

1. **完善火山引擎集成**
   - 在 `client/volcano_client.go` 中实现具体的 API 调用
   - 根据实际需求选择火山引擎的具体服务

2. **扩展规则引擎**
   - 在 `rules/engine.go` 中添加更多自定义规则
   - 实现复杂的业务逻辑判断

3. **添加测试**
   - 为每个模块编写单元测试
   - 添加集成测试

4. **性能优化**
   - 添加缓存机制
   - 实现并发处理
   - 添加监控和日志

5. **安全加固**
   - 添加认证授权
   - 实现请求限流
   - 添加输入验证

