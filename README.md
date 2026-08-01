# Go Pi Agent

一个使用 Go 从零实现的最小 AI Agent Runtime。

## MVP 目标

User Prompt → LLM Provider → Tool Call → Go Tool Execution → Tool Result → Final Answer

## 当前进度

- [x] 初始化 Go 项目
- [x] CLI 参数解析
- [x] Agent 核心消息模型
- [x] OpenAI-compatible Provider
- [x] read_file 工具
- [x] Tool Registry
- [x] Tool Calling 循环
- [x] SSE 流式响应
- [ ] 会话持久化
- [ ] PostgreSQL 工具
- [ ] MCP Client

## 运行
```bash
    go run ./cmd/agent "你好，请介绍一下自己"
```

## 当前 MVP 能力

- 支持 OpenAI-compatible Chat Completions API
- 支持模型自主选择工具
- 支持 read_file 文件读取工具
- 支持工具注册与统一执行
- 支持工具结果回填与多轮推理
- 支持最大运行轮次保护
- 支持工作目录路径越界保护
- 支持工具错误回填
- 支持 Token 用量统计
- 支持 .env 本地配置

## 示例

```bash
go run ./cmd/agent "读取 README.md，并总结这个项目"
```