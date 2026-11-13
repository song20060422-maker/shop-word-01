# Shop 部署与冒烟（题目 01）

## 1. 组成与架构（2026-11-13 拆分后，双服务）

| 组件 | 说明 |
| --- | --- |
| `server/Dockerfile.shop` → `shop-buyer` 镜像 | 买家端独立服务：`server/cmd/shop`（买家 + 模拟支付渠道，含全部契约路径），多阶段构建，静态编译，内置健康检查 |
| `server/Dockerfile.gva` → `shop-admin` 镜像 | GVA 管理端服务：gva 登录 + Casbin + `/shop/**` 后台接口，启动时自动迁移 gva 框架表 |
| `server/config.shop-docker.yaml` / `server/config.gva-docker.yaml` | 两服务容器配置，MySQL/Redis 用 compose 服务名寻址 |
| `deploy/shop/docker-compose.yaml` | MySQL 8 + Redis 7 + 买家端（18099）+ GVA 管理端（18098）+ admin-seed 一次性种子任务 |
| `deploy/shop/init/01-databases.sql`、`02-gva-framework.sql` | 数据卷首次初始化：建 gva 库 + gva 框架表结构与种子（mysqldump 自已初始化的库） |
| `deploy/shop/gva-admin-shop.sql` | 商城后台（运营/仓库/客服）菜单 / 接口 / Casbin 授权（幂等） |
| `deploy/shop/gva-framework-seed.sql` + `admin-seed.sh` | 已有数据卷场景的框架种子（sys_users 为空时注入） |

数据库约定：**业务表统一存放在 gva 库**（gva 框架表 + shop 业务 11 张表共用一个库），不再单独建 shop 库。
架构拆分：买家端 + 模拟支付渠道 = 独立 Gin 服务（`server/cmd/shop`，本地 :8090 / 容器 :18099）；
运营 / 仓库 / 客服后台 = GVA 管理端（gva 登录 + Casbin，接口 `/shop/**`，页面 `web/src/view/shop/**`，容器 :18098）。

## 2. 启动

```bash
# 版本标识：用提交短哈希作为镜像 tag，便于回溯
export SHOP_VERSION=$(git rev-parse --short HEAD)
docker compose -f deploy/shop/docker-compose.yaml up -d --build
docker compose -f deploy/shop/docker-compose.yaml ps
```

- 买家端容器启动时自动执行 `shop/migrations/*.sql`（建表 + 幂等种子），无需手工导库；
- GVA 管理端容器启动时自动迁移 gva 框架表；`admin-seed` 一次性服务会等框架表就绪后
  注入框架种子（admin/123456 等账号）与商城后台菜单/接口/Casbin 授权；
- **首次部署后需 `docker compose restart admin`**：Casbin 策略在启动时加载，种子注入后重启生效。
- 管理端页面：本地 `cd web && npm run dev`（或 `npm run build` 后由 nginx 等托管 `web/dist`）。

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

> 2026-11-12 22:20（batch3 整改会话）更新：远程仓库与 PR 已闭环（`song20060422-maker/shop-word-01`，PR #1/#2 + 分支保护）；部署已实际执行，见下节。

## 6. 实际部署记录（2026-11-12 22:18，batch3 整改会话）

1. 镜像构建：`docker compose -f deploy/shop/docker-compose.yaml build server`（`SHOP_VERSION=prerelease-9075881`，对应 dev/01-shop 部署配置修正提交）。
2. 启动：`docker compose up -d` → `shop-mysql`(13307) / `shop-redis`(16379) / `shop-server`(18099→8099) 全部 healthy。
3. 冒烟（对 `http://127.0.0.1:18099`，脚本 `docs/evidence/deploy-smoke.ps1`）：健康检查 / 买家登录 / 下单(10000 分) / 查单 WAIT_PAY / HMAC 支付通知 SUCCESS / 支付后 TO_SHIP —— **7/7 通过，退出码 0**，日志 `docs/evidence/deploy-smoke-20261112.log`。
4. 回退演练：git worktree 检出上一发布提交 `04d0f39` 构建镜像 `shop-server:release-04d0f39` → `up -d` 回退 → 复跑冒烟通过 → 恢复新版本（记录见 `docs/evidence/deploy-20261112.md`）。
5. 数据卷：`shop-mysql-data` 首次初始化执行 `init/01-databases.sql` 创建 `gva`/`shop` 两库；本机历史测试库容器（3307）已重命名为 `shop-mysql-test` 保留，与部署栈（13307）隔离。
