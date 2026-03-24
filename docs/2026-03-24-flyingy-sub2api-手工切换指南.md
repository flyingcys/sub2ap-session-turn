# flyingy-sub2api 手工切换指南

> 适用场景：你当前已有旧服务目录 `~/sub2api` 在运行，希望切换到当前项目目录 `/root/flyingcys/sub2ap-session-turn`，并避免名称冲突。

## 1. 目标

- 旧服务（`~/sub2api`）停止运行。
- 当前项目使用独立命名（`flyingy-sub2api`）启动，不与旧服务重名。
- 如需保留历史数据，可迁移旧目录中的 `data/postgres_data/redis_data`。

## 2. 当前项目关键命名（已完成）

当前项目 `deploy/*.yml` 已改为以下命名体系：

- Compose 项目名：`flyingy-sub2api`
- 主容器名：`flyingy-sub2api`
- 数据库容器名：`flyingy-sub2api-postgres`
- Redis 容器名：`flyingy-sub2api-redis`
- 默认镜像名：`flyingy-sub2api:latest`

## 3. 切换前准备

### 3.1 进入新项目并构建镜像

```bash
cd /root/flyingcys/sub2ap-session-turn

docker build -t flyingy-sub2api:latest -f Dockerfile .
```

### 3.2（可选但建议）备份旧部署目录

```bash
cd /root

tar czf sub2api-backup-$(date +%F-%H%M%S).tar.gz sub2api
```

## 4. 停掉旧服务（`~/sub2api`）

```bash
cd /root/sub2api/deploy

docker compose -f docker-compose.local.yml down
```

可选验证（确认旧容器已停止）：

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}' | rg 'sub2api|postgres|redis'
```

## 5. 启动当前项目服务

### 5.1 准备新项目 deploy 目录

```bash
cd /root/flyingcys/sub2ap-session-turn/deploy

# 若 .env 不存在，用模板生成
[ -f .env ] || cp .env.example .env
```

### 5.2 两种数据策略（二选一）

#### 策略 A：迁移旧数据（推荐，保留原账号/配置/数据库）

```bash
mkdir -p data postgres_data redis_data

# 拷贝旧环境变量（如你想沿用旧密码/密钥）
cp /root/sub2api/deploy/.env /root/flyingcys/sub2ap-session-turn/deploy/.env

# 迁移数据目录
rsync -a /root/sub2api/deploy/data/ /root/flyingcys/sub2ap-session-turn/deploy/data/
rsync -a /root/sub2api/deploy/postgres_data/ /root/flyingcys/sub2ap-session-turn/deploy/postgres_data/
rsync -a /root/sub2api/deploy/redis_data/ /root/flyingcys/sub2ap-session-turn/deploy/redis_data/
```

#### 策略 B：全新初始化（不迁移历史数据）

```bash
mkdir -p data postgres_data redis_data
cp -n .env.example .env

# 必改：设置数据库密码
# 编辑 .env，把 POSTGRES_PASSWORD 改成强密码
```

### 5.3 启动新服务

```bash
cd /root/flyingcys/sub2ap-session-turn/deploy

docker compose -f docker-compose.local.yml up -d
```

## 6. 切换后验证

### 6.1 查看容器状态

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' | rg 'flyingy-sub2api|sub2api'
```

期望看到：

- `flyingy-sub2api`
- `flyingy-sub2api-postgres`
- `flyingy-sub2api-redis`

### 6.2 看日志

```bash
cd /root/flyingcys/sub2ap-session-turn/deploy
docker compose -f docker-compose.local.yml logs -f sub2api
```

### 6.3 健康检查

```bash
curl -fsS http://127.0.0.1:8080/health && echo "OK"
```

## 7. 端口说明与修改（改为 8080）

### 7.1 当前默认端口

- 常规部署（`docker-compose.local.yml`）读取 `deploy/.env` 的 `SERVER_PORT`，默认是 `8080`。
- 测试复用栈（`docker-compose.reuse-stack-test.yml`）读取 `deploy/.env.test` 的 `TEST_SERVER_PORT`，默认是 `8081`。

### 7.2 修改为 8080

```bash
cd /root/flyingcys/sub2ap-session-turn/deploy

# 常规部署端口
sed -i 's/^SERVER_PORT=.*/SERVER_PORT=8080/' .env

# 若你使用 reuse-stack-test 测试栈，也改成 8080
sed -i 's/^TEST_SERVER_PORT=.*/TEST_SERVER_PORT=8080/' .env.test
```

### 7.3 修改后是否需要重新构建镜像

- 只改端口变量：不需要重新 `docker build`。
- 只需重建容器使端口映射生效：

```bash
# 常规部署
docker compose -f docker-compose.local.yml up -d

# 测试复用栈（按需）
docker compose -f docker-compose.reuse-stack-test.yml up -d
```

注意：

- 如果宿主机 `8080` 已被旧服务占用，先停掉旧服务再启动新服务。
- 只有在你改了代码或 Dockerfile 时，才需要重新构建镜像。

## 8. 回滚（新服务异常时）

```bash
# 1) 先停新服务
cd /root/flyingcys/sub2ap-session-turn/deploy
docker compose -f docker-compose.local.yml down

# 2) 再启动旧服务
cd /root/sub2api/deploy
docker compose -f docker-compose.local.yml up -d
```

## 9. 常用运维命令（新服务）

```bash
# 启动
cd /root/flyingcys/sub2ap-session-turn/deploy
docker compose -f docker-compose.local.yml up -d

# 停止
docker compose -f docker-compose.local.yml down

# 重启应用容器
docker compose -f docker-compose.local.yml restart sub2api

# 查看实时日志
docker compose -f docker-compose.local.yml logs -f sub2api
```

## 10. 备注

- 这套步骤是 Docker Compose 方案，不是 systemd 方案。
- 如果未来你改成 systemd，再使用 `deploy/flyingy-sub2api.service` 模板进行独立部署。
