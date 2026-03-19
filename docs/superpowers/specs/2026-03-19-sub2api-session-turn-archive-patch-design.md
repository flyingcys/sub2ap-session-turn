# sub2api 会话归档补丁设计

## 背景

当前仓库里已经有一套独立的 `archive proxy` 方案，目标是通过 `Nginx -> Python Proxy -> sub2api` 的形式保存对话请求与响应。

这个方案适合“不改 `sub2api` 源码”的部署，但不适合本轮目标。当前目标已经收敛为：

- 直接在 `sub2api` 内实现
- 以可长期维护的 `diff/patch` 形式跟随上游更新
- 按会话维度查看对话历史，而不是按日期或按单次请求零散归档
- 只关心客户端实际发送给 `sub2api` 的请求，以及客户端最终收到的响应
- 不关心 `sub2api` 到上游模型之间的内部改写与转发细节

因此，本轮不再继续扩展独立 `archive proxy`，而是在 `sub2api` 内设计一个最小侵入的“会话归档补丁”。

## 目标

实现后应满足以下目标：

1. 同一会话内的多轮请求与响应，始终归到同一个 session 目录下。
2. 同一会话内每一轮请求与响应，保存为一个独立文本文件，按 turn 顺序编号。
3. 归档内容只包含客户端视角的原始 HTTP 请求与最终 HTTP 响应。
4. 归档文件为明文文本，不压缩，不拆分 request/response 为两个文件。
5. 归档目录不按日期切分；跨天继续同一会话时，仍写入原 session 目录。
6. 改动尽量集中在少量 handler 挂点和一个独立归档包中，便于长期维护为 patch。
7. 同时覆盖 Codex/OpenAI 与 Claude 两类对话入口。

## 非目标

以下内容不在本轮范围内：

- 不保存 `sub2api -> 上游模型` 的内部转发请求与响应
- 不保存额外分析摘要、结构化提取结果或对话重建结果
- 不做数据库存储
- 不做 Web 管理界面
- 不做归档压缩
- 不按项目名、token 名、日期层级进行目录组织
- 不把 WebSocket 帧级归档纳入首版补丁

## 用户确认的核心约束

本设计以以下已确认约束为准：

- 归档位置应直接放在 `sub2api` 内，而不是独立代理项目
- 归档应按 session 聚合，而不是按日期聚合
- session 目录名不直接使用原始 `session_id`，而使用稳定映射值
- 每轮对话只生成一个文本文件，例如 `0001.txt`
- 文件内容只保存原始 request 与 response
- 需要同时支持：
  - `POST /v1/messages`
  - `POST /v1/responses`
  - `POST /responses`
  - `POST /chat/completions`
- 归档应适合长期维护为单个或少量 patch，而不是做成强耦合的大功能分支

## 方案对比

### 方案一：继续扩展独立 archive proxy

优点：

- 与 `sub2api` 仓库隔离
- 不直接修改主项目源码

缺点：

- 需要复制 `sub2api` 的会话判定逻辑
- 很难准确对齐 `sub2api` 当前支持的入口和兼容链路
- 长期会演变为第二套归档系统
- 用户最终要维护的是 `sub2api` 更新后的行为，而不是旁路代理

结论：

- 不推荐

### 方案二：在 `sub2api` 中做完整可配置功能

优点：

- 功能可以做得完整
- 可通过配置启停

缺点：

- 改动面大
- 会触及配置、文档、部署、管理逻辑
- 与“保持 patch 维护成本低”的目标冲突

结论：

- 不推荐作为首版

### 方案三：在 `sub2api` 中做最小侵入补丁

做法：

- 新增一个独立归档包
- 在少量 handler 入口做 request 捕获与 session 解析
- 在最终 response 写回客户端的链路上统一捕获输出
- 在请求结束时把本轮 request/response 落成一个 `turn` 文件

优点：

- 改动边界清晰
- patch 面最小
- 最符合长期跟随 `sub2api` 更新的诉求

缺点：

- 首版功能边界需要严格收敛
- 不会覆盖所有协议形态

结论：

- 推荐采用

## 总体设计

### 设计原则

- 只做“客户端视角归档”，不做内部转发审计
- 只挂接必要入口，不改动路由结构
- 把新增逻辑收拢到独立包，减少对原有 service/handler 的侵入
- 首版优先覆盖 HTTP 与 SSE 主路径
- 不按日期切目录，确保一个 session 可长期连续追加
- 每个 turn 独立文件，避免同一 session 并发写入互相穿插

### 目录模型

归档目录结构如下：

```text
archive/
  sess_<stable-hash>/
    0001.txt
    0002.txt
    0003.txt
```

说明：

- `sess_<stable-hash>`
  - 由原始 session 语义生成稳定映射值
  - 不直接暴露原始 `session_id` / `conversation_id`
- `0001.txt`
  - 表示该 session 的第 1 个 turn
  - 一个 turn 文件内同时包含本轮 request 和 response
- 跨天不切目录
  - 只要 session 语义仍然一致，就继续写入同一目录

### turn 文件格式

每个 `turn` 文件都是纯文本，建议格式如下：

```text
===== REQUEST =====
POST /responses HTTP/1.1
content-type: application/json
session_id: ...
conversation_id: ...
x-request-id: ...

{原始请求体}

===== RESPONSE =====
HTTP/1.1 200 OK
content-type: text/event-stream
x-request-id: ...

{最终返回给客户端的原始响应体}
```

说明：

- 文件中保留最少但足够判断链路的 HTTP 头
- 不额外写 `meta.json`
- 不做人类摘要
- 不做 gzip 压缩

## session 识别规则

session 识别不重新发明，直接复用 `sub2api` 当前已有的会话语义。

### OpenAI/Codex 链路

优先级与 `OpenAIGatewayService.GenerateSessionHash()` 和 `ExtractSessionID()` 对齐：

1. `session_id` header
2. `conversation_id` header
3. request body 中的 `prompt_cache_key`

对应现有实现位置：

- `backend/internal/service/openai_gateway_service.go`

### Claude `/v1/messages` 链路

对于 Anthropic/Claude 兼容入口，除了复用 header 与 body 中已有会话信号外，再补一个与现有处理语义一致的兜底来源：

1. `session_id` header
2. `conversation_id` header
3. request body 中的 `prompt_cache_key`
4. request body 中的 `metadata.user_id`
5. 兜底为单请求临时 session

原因：

- `sub2api` 当前会在 OpenAI Anthropic 兼容入口中从 `metadata.user_id` 派生 session 语义
- 这能更稳定地把 Claude Code 多轮请求归到同一 session

### session 目录名

session 目录名不直接使用原始 session 文本，而使用稳定映射值，例如：

```text
sess_a13f5c2d7b9e...
```

推荐做法：

- 对“原始 session key”做哈希
- 统一加固定前缀 `sess_`
- 如需排障，可在文件内容头部写出原始 session 相关头，但不作为目录名

## turn 分配规则

### turn 的定义

在本补丁里：

- 一个 turn = 同一 session 下的一次独立 HTTP 请求及其对应响应

这意味着：

- 一次请求只会生成一个 turn 文件
- 文件里包含 request 和 response 两部分
- 不会把 request 和 response 拆成两个文件

### 并发处理

同一 session 下允许出现并发请求，归档方式如下：

- 每个进入该 session 的请求都分配新的 turn 号
- 每个请求单独写入一个 `000N.txt`
- 不把多个请求串写进同一个 session 大文件

这样做的目的是：

- 避免并发写入交错
- 避免一个 session 文件无限增长
- 更适合后续做 Git 备份与差异查看

### turn 号分配

建议 turn 号在 session 目录下原子分配，保证：

- 同一 session 内单调递增
- 并发请求不会拿到同一个编号
- 重启后仍可从现有目录继续分配下一个 turn

可接受的实现策略：

- 通过 session 级文件锁扫描现有文件号后分配
- 或维护 session 级计数文件并用文件锁保护

首版推荐：

- 使用 session 级锁 + 扫描现有 `*.txt` 文件获取下一编号

优点是：

- 结构简单
- patch 面更小
- 不需要额外引入状态文件格式

## 归档挂点设计

### 目标挂点

补丁只接入以下 handler：

- `backend/internal/handler/openai_gateway_handler.go`
  - `Responses`
  - `Messages`
- `backend/internal/handler/openai_chat_completions.go`
  - `ChatCompletions`
- `backend/internal/handler/gateway_handler.go`
  - `Messages`

原因：

- 这些是当前目标范围内真正的客户端入口
- 绝大多数 request body 都在这些 handler 开头被读取
- 最终响应也都会经由当前 `gin.Context.Writer` 写回客户端

### request 捕获

在 handler 中读取完 request body 且完成最基本合法性校验后：

- 解析 session key
- 创建归档 recorder
- 记录原始 request line、必要 headers、原始 body

request 捕获以“客户端发给 `sub2api` 的原始请求”为准，不记录内部改写后的上游请求。

### response 捕获

为尽量减少对 service 层的侵入，推荐在 handler 入口把 `c.Writer` 包装为带镜像能力的 writer：

- 正常写给客户端
- 同时把写出的内容复制到内存 buffer 或受限 buffer

需要覆盖的最小能力：

- `Write`
- `WriteString`
- `WriteHeader`
- `Header`
- `Flush`
- `Status`
- `Written`
- `Size`

设计目标是：

- 不改变现有 handler/service 对 `gin.ResponseWriter` 的使用方式
- 无论是 JSON、普通 HTTP body 还是 SSE 输出，都能捕获到最终发给客户端的内容

### 落盘时机

推荐在 handler 返回前通过 `defer` 完成最终落盘：

- 已记录 request
- response writer 已累计本轮实际响应内容
- 可从 writer 中读取最终状态码和响应头
- 然后一次性写入 turn 文件

这样能把改动控制在入口 handler 附近，而不是分散改动多个 service 的返回路径。

## 组件设计

### `session_resolver`

职责：

- 从 `gin.Context` 和 request body 提取 session 原始标识
- 按当前规则生成稳定 session key
- 返回：
  - 原始 session key
  - 目录名使用的 hash key

### `turn_allocator`

职责：

- 在某个 session 目录下分配下一个 turn 号
- 处理并发互斥

### `response_capture_writer`

职责：

- 包装 `gin.ResponseWriter`
- 捕获最终写给客户端的状态码、响应头、body
- 对 SSE 与普通响应保持透明

### `archive_recorder`

职责：

- 聚合 request 与 response 数据
- 在 handler 生命周期结束时生成最终文本内容
- 调用 store 持久化

### `archive_store`

职责：

- 创建目录
- 写 turn 文件
- 保证写入原子性

## 错误处理

错误处理遵循以下原则：

- 归档失败不能影响主请求成功返回
- 任何归档异常只记录日志，不回写给客户端
- session 解析失败时，退化为单请求临时 session
- turn 分配失败时，允许降级使用时间戳随机文件名，但应记录错误日志
- response 捕获失败时，不影响正常响应写出

## 性能与资源

考虑到用户服务器资源有限，补丁需要严格控制内存与磁盘开销。

### 资源原则

- 只缓存当前请求的 request/response 文本
- 不做额外 JSON 解析结果持久化
- 不做 gzip 压缩
- 不做异步队列和后台 worker

### 风险

由于目标是保存“原始客户端视角内容”，response 需要完整捕获后再落盘，因此：

- 长响应会增加本轮内存占用
- SSE 长流会在内存中积累较大文本

首版接受这个取舍，原因是：

- 用户实际并发较低
- 目标是得到完整可回看的文本记录
- 与引入复杂的流式边写边拼接文件相比，这种方式 patch 面更小

后续若需要进一步收敛内存，可再补：

- response 捕获大小限制
- 流式边写临时文件再封口

但这些不属于首版补丁范围。

## 与 patch 维护目标的关系

本设计专门服务“长期维护 diff 文件”的诉求，因此约束如下：

- 不修改现有路由定义
- 不新增配置中心字段
- 不引入数据库迁移
- 不改前端
- 不改现有部署脚本
- 不改业务调度核心逻辑
- 只新增独立归档包，并在少数 handler 里插入调用

理想的最终改动范围应大致控制在：

- 新增一个独立目录，例如 `backend/internal/pkg/conversationarchive/`
- 修改 3 到 4 个 handler 文件
- 新增对应测试

这样更适合保持为单提交补丁，后续可通过：

```bash
git format-patch
```

持续导出与重放。

## 测试策略

至少补以下测试：

1. session 识别优先级
   - `session_id`
   - `conversation_id`
   - `prompt_cache_key`
   - `metadata.user_id`

2. 同一 session 跨请求追加
   - 首次生成 `0001.txt`
   - 再次请求生成 `0002.txt`

3. 同一 session 并发请求
   - 能分配不同 turn 号
   - 不会互相覆盖

4. turn 文件格式
   - 包含 request 段
   - 包含 response 段
   - 包含状态行与必要头

5. SSE 响应归档
   - 文件中保留最终发给客户端的 SSE 文本

6. 归档失败不影响主请求
   - 写盘失败时主响应仍然成功

7. 跨天续写同一 session
   - 不引入日期目录
   - 继续在原 session 目录追加 turn 文件

## 风险与控制

### 风险一：ResponseWriter 包装与现有流式路径不兼容

控制方式：

- 优先实现最小透明 writer 包装
- 用现有流式测试与新增归档测试共同验证

### 风险二：同一 session 并发 turn 分配冲突

控制方式：

- 明确使用 session 级锁
- 用并发测试验证编号唯一性

### 风险三：长 SSE 响应导致内存增长

控制方式：

- 首版接受低并发下的内存成本
- 设计上预留后续改为临时文件流式落盘的空间

### 风险四：补丁长期跟随上游更新时冲突

控制方式：

- 改动集中在新增包与少量 handler
- 不碰路由与核心调度逻辑
- 不扩散到配置、管理台和部署层

## 结论

推荐在 `sub2api` 中以“最小侵入补丁”的方式实现会话归档：

- 归档范围限定为客户端视角 request/response
- 归档组织采用 `session -> turn`
- 每个 turn 一个 `000N.txt`
- 目录不按日期切分
- session 目录名使用稳定 hash
- 首版仅覆盖 HTTP/SSE 主路径
- 改动集中在独立归档包与少数 handler 挂点

该方案最符合当前目标：

- 数据清晰
- 对话可回看
- 会话可连续追加
- 并发不混写
- 适合长期维护为 patch
