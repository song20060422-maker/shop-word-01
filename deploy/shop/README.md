# Shop 部署与冒烟（题目 01）

## 1. 组成

| 组件 | 说明 |
| --- | --- |
| `server/Dockerfile.shop` | Shop 单服务镜像（多阶段构建，静态编译，内置健康检查） |
| `server/config.shop-docker.yaml` | 容器内配置：MySQL/Redis 用服务名寻址，业务配置段 `shop.*` |
| `deploy/shop/docker-compose.yaml` | MySQL 8 + Redis 7 + Shop 服务，端口 18099 / 13307 / 16379 |

## 2. 启动

```bash
# 版本标识：用提交短哈希作为镜像 tag，便于回溯
export SHOP_VERSION=$(git rev-parse --short HEAD)
docker compose -f deploy/shop/docker-compose.yaml up -d --build
docker compose -f deploy/shop/docker-compose.yaml ps
```

服务启动时自动执行 `shop/migrations/*.sql`（建表 + 幂等种子），无需手工导库。

## 3. 冒烟

```bash
curl -fsS http://127.0.0.1:18099/api/health
# {"module":"shop","status":"ok"}

# 页面
curl -fsS http://127.0.0.1:18099/shop -o /dev/null -w "%{http_code}\n"
```

完整冒烟脚本：`server/scripts/ci-smoke.sh`（Linux/macOS）或
`server/scripts/shop-smoke.ps1`（Windows，本机开发态）。

## 4. 版本与回滚

| 项 | 做法 |
| --- | --- |
| 版本标识 | 镜像 tag = 提交短哈希（`SHOP_VERSION`），容器内 `GET /api/health` 可用于存活探测 |
| 数据库变更 | 迁移脚本只增不改、全部 `IF NOT EXISTS`，旧版本回滚不需要回滚表结构 |
| 回滚 | `docker compose down` 后用上一版本 tag 重新 `up -d`（`image: shop-server:<旧哈希>`） |
| 数据 | MySQL 数据卷 `shop-mysql` 独立，回滚镜像不影响数据 |

## 5. 未完成项（如实标注）

- 视频演示：未录制（需学生出镜讲解业务与并发验证）。
- 群提交：未提交（需学生按群公告格式发送仓库地址与验收记录）。
- 远程仓库与 PR：当前工作区无可用远程（`github.com` 直连超时），推送到远端后由 CI 自动跑质量门禁。
