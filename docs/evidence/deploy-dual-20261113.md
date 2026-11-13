# 双服务镜像重建与部署留证（I-25 整改）

- 日期：2026-11-13
- 提交：`8b0d468`（dev/01-shop，数据库合一 + 前后台服务拆分 + adminapi/api 测试补齐）
- 镜像 tag：`SHOP_VERSION=8b0d468`

## 1. 新增部署资产

| 文件 | 作用 |
| --- | --- |
| `server/Dockerfile.gva` | GVA 管理端镜像（gva 登录 + Casbin + /shop/** 后台接口） |
| `server/config.gva-docker.yaml` | 管理端容器配置（mysql/redis 服务名寻址） |
| `deploy/shop/gva-framework-seed.sql` | gva 框架种子数据（admin 账号/角色/菜单/API/Casbin，mysqldump 自已初始化库） |
| `deploy/shop/init/02-gva-framework.sql` | 数据卷首初始化：gva 框架表结构 + 种子 |
| `deploy/shop/admin-seed.sh` | 一次性种子任务脚本（等框架表就绪 → 框架种子 → 商城后台授权，幂等） |
| `deploy/shop/docker-compose.yaml` | 扩为双服务：shop-buyer(18099) + shop-admin(18098) + admin-seed |

镜像 tag 由 `shop-server:release-832cda4`（旧架构）换为 `shop-buyer:8b0d468` + `shop-admin:8b0d468`。

## 2. 部署过程实录

```
$ SHOP_VERSION=8b0d468 docker compose -f deploy/shop/docker-compose.yaml up -d --build
 Container shop-admin Healthy
 Container shop-admin-seed Started
$ docker logs shop-admin-seed   # 首次因等待条件不当失败，修正为等待 sys_users 表后重跑
[admin-seed] 等待 gva 框架表初始化...
[admin-seed] 注入 gva 框架种子数据...
[admin-seed] 商城后台菜单/接口/授权就绪：
42                       # /shop/** Casbin 授权策略数
$ docker compose -f deploy/shop/docker-compose.yaml restart admin   # Casbin 启动时加载，重启生效
```

注：为排除旧数据卷残留（首轮种子顺序冲突），执行过 `docker compose down -v` 重置数据卷后
重新部署，gva 框架表/种子与商城授权均为干净初始化。

## 3. 冒烟结果（容器环境）

| 检查 | 结果 |
| --- | --- |
| 买家端 `http://127.0.0.1:18099/api/health` | 200 `{"module":"shop","status":"ok"}` |
| 买家端页面 `/shop` | 200（14853 字节 buyer.html） |
| 买家端重置/登录/下单 | code=0，下单返回 SKU-A amount=10000 |
| 管理端 `http://127.0.0.1:18098/health` | 200 |
| 管理端登录 `/base/login`（admin） | code=0，token 正常签发 |
| `/shop/ops/summary`（x-token） | code=0（orderCount 实时数据） |
| `/shop/ops/products`（x-token） | code=0，返回 SKU-A/SKU-B |

## 4. 版本锁定

- 买家端镜像：`shop-buyer:8b0d468`；管理端镜像：`shop-admin:8b0d468`
- 容器：`shop-buyer`、`shop-admin`、`shop-admin-seed`（一次性）
- `docker compose -f deploy/shop/docker-compose.yaml ps` 可复核镜像 tag 与健康状态
