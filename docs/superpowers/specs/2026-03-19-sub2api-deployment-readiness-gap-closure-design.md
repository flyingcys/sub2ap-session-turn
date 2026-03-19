# sub2api 部署就绪缺口收口设计

## 背景

本次收口针对三类会直接影响上线判断的问题：

1. 会话归档默认目录没有落到持久化卷内
2. 部署模板没有显式声明会话归档目录
3. 归档与支付幂等文档存在事实分叉

## 目标

- 让会话归档默认写入 `data/archive/conversations`
- 让 Docker / systemd 模板显式声明归档目录
- 废弃旧的外置 Python 代理文档，提供当前实现的正式部署说明
- 把支付幂等默认值与生产建议写清楚，避免文档误导

## 方案

### 1. 归档目录默认值

将会话归档默认根目录从：

```text
archive/conversations
```

改为：

```text
data/archive/conversations
```

这样在 Docker 中会自然落到 `/app/data/...`，在二进制部署中会自然落到 `/opt/sub2api/data/...`。

### 2. 部署模板

在以下模板中显式写入 `SUB2API_CONVERSATION_ARCHIVE_ROOT`：

- `deploy/docker-compose.yml`
- `deploy/docker-compose.local.yml`
- `deploy/docker-compose.dev.yml`
- `deploy/sub2api.service`
- `deploy/install.sh`
- `deploy/.env.example`

这样即使未来默认值再调整，正式部署模板也不会漂移。

### 3. 文档统一

- 在 `docs/2026-03-18-*` 文档顶部加“已废弃”说明
- 新增 `docs/2026-03-19-sub2api-会话归档部署说明.md`
- 更新 `docs/ADMIN_PAYMENT_INTEGRATION_API.md`
- 更新 `deploy/config.example.yaml` 中的幂等说明

## 风险与约束

- 不把 `idempotency.observe_only` 全局默认改成 `false`
  原因是这会影响所有接入幂等保护的后台写接口，当前前端并未全面发送 `Idempotency-Key`
- 旧文档位于 `docs/*` 根目录，需要放开 `.gitignore` 例外规则，否则无法纳入版本控制
