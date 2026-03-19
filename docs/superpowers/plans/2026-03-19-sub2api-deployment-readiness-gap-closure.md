# sub2api 部署就绪缺口收口 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 收口会话归档持久化、部署模板和文档分叉三类部署阻断项

**Architecture:** 先用测试锁定会话归档默认落盘目录，再同步部署模板与配置示例，最后统一历史文档与当前正式部署文档。保持业务逻辑最小改动，以部署一致性和可验证性为优先。

**Tech Stack:** Go, Gin, Docker Compose, systemd, Markdown

---

### Task 1: 锁定默认归档目录

**Files:**
- Modify: `backend/internal/handler/conversation_archive_test.go`
- Modify: `backend/internal/handler/conversation_archive.go`

- [ ] **Step 1: 先写失败测试**

新增测试，断言未设置 `SUB2API_CONVERSATION_ARCHIVE_ROOT` 时，归档写入 `data/archive/conversations`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./backend/internal/handler -run TestConversationArchiveHelpers_UseDataArchiveRootByDefault -count=1`
Expected: FAIL，找不到 `data/archive/conversations/...`

- [ ] **Step 3: 修改默认目录实现**

把默认根目录从 `archive/conversations` 改为 `data/archive/conversations`。

- [ ] **Step 4: 重新运行测试确认通过**

Run: `go test ./backend/internal/handler -run 'TestConversationArchiveHelpers_(PersistTurnFile|UseDataArchiveRootByDefault)' -count=1`
Expected: PASS

### Task 2: 收敛部署模板

**Files:**
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/docker-compose.local.yml`
- Modify: `deploy/docker-compose.dev.yml`
- Modify: `deploy/.env.example`
- Modify: `deploy/sub2api.service`
- Modify: `deploy/install.sh`

- [ ] **Step 1: 写入归档目录环境变量**

统一写入：

- Docker：`/app/data/archive/conversations`
- systemd：`/opt/sub2api/data/archive/conversations`

- [ ] **Step 2: 校验 compose 模板**

Run: `docker compose -f deploy/docker-compose.yml config`
Expected: PASS

- [ ] **Step 3: 校验 local/dev 模板**

Run: `docker compose -f deploy/docker-compose.local.yml config`
Run: `docker compose -f deploy/docker-compose.dev.yml config`
Expected: PASS

### Task 3: 统一归档与支付文档

**Files:**
- Modify: `.gitignore`
- Modify: `docs/2026-03-18-sub2api-对话归档最终方案.md`
- Modify: `docs/2026-03-18-sub2api-对话归档实施计划.md`
- Modify: `docs/2026-03-18-sub2api-对话归档实施说明.md`
- Create: `docs/2026-03-19-sub2api-会话归档部署说明.md`
- Modify: `docs/ADMIN_PAYMENT_INTEGRATION_API.md`
- Modify: `deploy/config.example.yaml`

- [ ] **Step 1: 放开根目录文档跟踪**

在 `.gitignore` 中为本次需要纳入版本控制的 `docs/*.md` 添加例外规则。

- [ ] **Step 2: 给旧文档加废弃说明**

明确 2026-03-18 的 Python 代理方案已废弃，仅保留历史背景。

- [ ] **Step 3: 新增正式部署说明**

覆盖：

- 当前实现架构
- 默认落盘路径
- Docker / systemd 配置方式
- 验证步骤
- 当前限制与回滚方式

- [ ] **Step 4: 修正支付幂等文档**

明确：

- 默认 `observe_only=true`
- 支付生产建议切到 `false`
- 这是全局开关，切换前要确认调用方都带 `Idempotency-Key`

### Task 4: 完整验证

**Files:**
- Verify: `backend/internal/handler`
- Verify: `backend/internal/pkg/conversationarchive`
- Verify: `deploy/docker-compose*.yml`
- Verify: `deploy/sub2api.service`

- [ ] **Step 1: 运行后端相关测试**

Run: `go test ./backend/internal/pkg/conversationarchive/... ./backend/internal/handler -run 'ConversationArchive|CreateAndRedeem' -count=1`
Expected: PASS

- [ ] **Step 2: 运行后端编译**

Run: `go build -buildvcs=false ./backend/cmd/server`
Expected: PASS

- [ ] **Step 3: 校验部署模板**

Run: `docker compose -f deploy/docker-compose.yml config`
Run: `docker compose -f deploy/docker-compose.local.yml config`
Run: `docker compose -f deploy/docker-compose.dev.yml config`
Expected: PASS
