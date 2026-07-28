# Go Pi Agent

一个使用 Go 从零实现的最小 AI Agent Runtime。

## MVP 目标

User Prompt → LLM Provider → Tool Call → Go Tool Execution → Tool Result → Final Answer

## 当前进度

- [x] 初始化 Go 项目
- [x] CLI 参数解析
- [x] Agent 核心消息模型
- [ ] OpenAI-compatible Provider
- [x] read_file 工具
- [x] Tool Registry
- [ ] Tool Calling 循环
- [ ] SSE 流式响应
- [ ] 会话持久化
- [ ] PostgreSQL 工具
- [ ] MCP Client

## 运行

    go run ./cmd/agent "你好，请介绍一下自己"
