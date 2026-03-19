# sub2api 会话归档补丁 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `sub2api` 内实现一个适合长期维护为 patch 的会话归档补丁，按 `session -> turn` 把客户端视角的 request/response 保存为文本文件。

**Architecture:** 新增独立的 `conversationarchive` 包，负责 session 解析、turn 分配、response 捕获和最终落盘；在 OpenAI/Claude 相关 handler 入口做最小接入，通过包装 `gin.ResponseWriter` 捕获最终返回给客户端的响应内容。整个实现不改路由、不改配置、不改数据库，只在少量 handler 文件插桩。

**Tech Stack:** Go, Gin, 标准库文件锁/文件写入, 现有 handler/service 测试体系

---

### Task 1: 建立归档核心包的失败测试

**Files:**
- Create: `backend/internal/pkg/conversationarchive/session_test.go`
- Create: `backend/internal/pkg/conversationarchive/store_test.go`
- Create: `backend/internal/pkg/conversationarchive/response_capture_writer_test.go`
- Create: `backend/internal/pkg/conversationarchive/doc.go`

- [ ] **Step 1: 写 session 解析失败测试**

```go
func TestResolveSessionKey_Priority(t *testing.T) {
    // session_id > conversation_id > prompt_cache_key > metadata.user_id
}
```

- [ ] **Step 2: 运行 session 测试确认失败**

Run: `go test ./backend/internal/pkg/conversationarchive -run TestResolveSessionKey_Priority -v`
Expected: FAIL，提示包或函数尚不存在

- [ ] **Step 3: 写 turn 分配和落盘失败测试**

```go
func TestAllocateNextTurn_Sequential(t *testing.T) {}
func TestWriteTurnFile_CreatesExpectedFormat(t *testing.T) {}
```

- [ ] **Step 4: 运行 store 测试确认失败**

Run: `go test ./backend/internal/pkg/conversationarchive -run 'TestAllocateNextTurn_Sequential|TestWriteTurnFile_CreatesExpectedFormat' -v`
Expected: FAIL，提示实现缺失

- [ ] **Step 5: 写 response writer 透明捕获失败测试**

```go
func TestCaptureWriter_MirrorsBodyAndStatus(t *testing.T) {}
func TestCaptureWriter_SupportsFlush(t *testing.T) {}
```

- [ ] **Step 6: 运行 response writer 测试确认失败**

Run: `go test ./backend/internal/pkg/conversationarchive -run 'TestCaptureWriter_' -v`
Expected: FAIL，提示实现缺失

### Task 2: 实现归档核心包

**Files:**
- Create: `backend/internal/pkg/conversationarchive/session.go`
- Create: `backend/internal/pkg/conversationarchive/store.go`
- Create: `backend/internal/pkg/conversationarchive/response_capture_writer.go`
- Create: `backend/internal/pkg/conversationarchive/recorder.go`
- Modify: `backend/internal/pkg/conversationarchive/session_test.go`
- Modify: `backend/internal/pkg/conversationarchive/store_test.go`
- Modify: `backend/internal/pkg/conversationarchive/response_capture_writer_test.go`

- [ ] **Step 1: 实现 session 解析**

```go
type SessionInput struct {
    Headers http.Header
    Body    []byte
    Path    string
}

func ResolveSession(input SessionInput) ResolvedSession
```

- [ ] **Step 2: 运行 session 测试确认通过**

Run: `go test ./backend/internal/pkg/conversationarchive -run TestResolveSessionKey_Priority -v`
Expected: PASS

- [ ] **Step 3: 实现 turn 分配和文本落盘**

```go
func (s *Store) AllocateTurn(ctx context.Context, sessionDir string) (int, error)
func (s *Store) WriteTurn(ctx context.Context, record TurnRecord) (string, error)
```

- [ ] **Step 4: 运行 store 测试确认通过**

Run: `go test ./backend/internal/pkg/conversationarchive -run 'TestAllocateNextTurn_Sequential|TestWriteTurnFile_CreatesExpectedFormat' -v`
Expected: PASS

- [ ] **Step 5: 实现 response capture writer**

```go
type CaptureWriter struct {
    gin.ResponseWriter
}
```

- [ ] **Step 6: 运行 response writer 测试确认通过**

Run: `go test ./backend/internal/pkg/conversationarchive -run 'TestCaptureWriter_' -v`
Expected: PASS

### Task 3: 在 OpenAI Responses 和 Chat Completions 入口接入归档

**Files:**
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/handler/openai_gateway_handler_test.go`
- Create: `backend/internal/handler/openai_archive_integration_test.go`

- [ ] **Step 1: 为 OpenAI handler 写失败测试**

```go
func TestResponsesArchive_WritesTurnFile(t *testing.T) {}
func TestChatCompletionsArchive_WritesTurnFile(t *testing.T) {}
```

- [ ] **Step 2: 运行 OpenAI 归档测试确认失败**

Run: `go test ./backend/internal/handler -run 'TestResponsesArchive_WritesTurnFile|TestChatCompletionsArchive_WritesTurnFile' -v`
Expected: FAIL，尚未接入归档

- [ ] **Step 3: 在 `Responses` 与 `ChatCompletions` handler 包装 capture writer 并接入 recorder**

```go
recorder := conversationarchive.Start(...)
defer recorder.Finish(...)
```

- [ ] **Step 4: 运行 OpenAI 归档测试确认通过**

Run: `go test ./backend/internal/handler -run 'TestResponsesArchive_WritesTurnFile|TestChatCompletionsArchive_WritesTurnFile' -v`
Expected: PASS

### Task 4: 在 Claude/OpenAI Messages 入口接入归档

**Files:**
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Create: `backend/internal/handler/messages_archive_integration_test.go`

- [ ] **Step 1: 为 `GatewayHandler.Messages` 和 `OpenAIGatewayHandler.Messages` 写失败测试**

```go
func TestGatewayMessagesArchive_UsesMetadataUserIDAsFallback(t *testing.T) {}
func TestOpenAIMessagesArchive_WritesFinalResponse(t *testing.T) {}
```

- [ ] **Step 2: 运行 Messages 归档测试确认失败**

Run: `go test ./backend/internal/handler -run 'TestGatewayMessagesArchive_|TestOpenAIMessagesArchive_' -v`
Expected: FAIL，尚未接入归档

- [ ] **Step 3: 在两个 Messages handler 接入 recorder**

```go
resolved := conversationarchive.ResolveSession(...)
writer := conversationarchive.WrapWriter(c.Writer)
```

- [ ] **Step 4: 运行 Messages 归档测试确认通过**

Run: `go test ./backend/internal/handler -run 'TestGatewayMessagesArchive_|TestOpenAIMessagesArchive_' -v`
Expected: PASS

### Task 5: 处理并发、跨天与异常容错

**Files:**
- Modify: `backend/internal/pkg/conversationarchive/store_test.go`
- Modify: `backend/internal/pkg/conversationarchive/store.go`
- Modify: `backend/internal/pkg/conversationarchive/recorder.go`
- Create: `backend/internal/pkg/conversationarchive/recorder_test.go`

- [ ] **Step 1: 为并发 turn 分配和归档失败不影响主响应写失败测试**

```go
func TestAllocateNextTurn_Concurrent(t *testing.T) {}
func TestRecorder_FinishIgnoresArchiveError(t *testing.T) {}
```

- [ ] **Step 2: 运行相关测试确认失败**

Run: `go test ./backend/internal/pkg/conversationarchive -run 'TestAllocateNextTurn_Concurrent|TestRecorder_FinishIgnoresArchiveError' -v`
Expected: FAIL

- [ ] **Step 3: 实现 session 级锁、异常吞吐与跨进程兼容写入**

```go
// 使用 session 级 mutex / lock file 分配 turn
```

- [ ] **Step 4: 运行相关测试确认通过**

Run: `go test ./backend/internal/pkg/conversationarchive -run 'TestAllocateNextTurn_Concurrent|TestRecorder_FinishIgnoresArchiveError' -v`
Expected: PASS

### Task 6: 文档与验证

**Files:**
- Modify: `docs/superpowers/specs/2026-03-19-sub2api-session-turn-archive-patch-design.md`
- Create: `docs/superpowers/specs/2026-03-19-sub2api-session-turn-archive-patch-notes.md`（仅当需要记录实现偏差时）

- [ ] **Step 1: 运行归档包测试**

Run: `go test ./backend/internal/pkg/conversationarchive/...`
Expected: PASS

- [ ] **Step 2: 运行 handler 相关测试**

Run: `go test ./backend/internal/handler -run 'Archive|OpenAI|Messages' -count=1`
Expected: PASS

- [ ] **Step 3: 运行核心 service/streaming 回归**

Run: `go test ./backend/internal/service -run 'Streaming|OpenAIGatewayService|GatewayService' -count=1`
Expected: PASS

- [ ] **Step 4: 运行更完整的回归验证**

Run: `go test ./backend/internal/handler ./backend/internal/service ./backend/internal/pkg/conversationarchive/... -count=1`
Expected: PASS

- [ ] **Step 5: 整理补丁说明**

```bash
git status --short
git diff --stat
```

- [ ] **Step 6: 提交**

```bash
git add .gitignore docs/superpowers/specs docs/superpowers/plans backend/internal/pkg/conversationarchive backend/internal/handler
git commit -m "feat: archive conversations by session and turn"
```
