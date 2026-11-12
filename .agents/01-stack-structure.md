# 01 · 技术栈与目录结构

## 技术栈

| 项 | 值 |
|---|---|
| 框架 | gin-vue-admin v2.9.2（Go + Gin + GORM） |
| Go | 1.26.x（本地工具链） |
| 数据库 | MySQL 8（3307；业务库 `shop`，框架库 `gva`） |
| 服务端口 | 8099（shop 段 `addr`） |
| 商城页面 | `server/shop/web/index.html`（后端直出的单页，非 Vue 前端） |

## 目录分层与职责边界

| 层 | 路径 | 职责 | 禁止 |
|---|---|---|---|
| api | `server/shop/api/` | 绑定、身份、错误码映射 | 不写事务与状态判断 |
| service | `server/shop/service/` | 事务、状态机、库存守恒、渠道、审计 | handler 不改状态字段 |
| model | `server/shop/model/` | 表、常量、DTO | 金额禁止浮点 |
| middleware | `server/shop/middleware/` | JWT、角色门禁（RequireRole） | 角色只信 token |
| initialize | `server/shop/initialize/` | 启动装配、迁移、种子 | 迁移幂等，失败即启动失败 |
| db | `server/shop/db/` | DB 连接构建（独立包，消除 initialize↔service 导入环） | — |
| migrations | `server/shop/migrations/*.sql` | 建表与预置数据 | 已发布脚本不回改 |
| router | `server/shop/router/` | 公开/登录/角色分组、契约兼容路由 | 不自造冲突路径 |
| web | `server/shop/web/` | 内置演示页（5 标签） | 不引入前端构建链 |
| utils | `server/shop/utils/` | 单号、HMAC、bcrypt | 密钥不入库、不打日志 |

## 允许改动的框架边界

- 只允许改：`server/initialize/router_biz.go`（装配）、`server/config.yaml` 的 `shop` 段。
- 不改 GVA `system` 模块；不另起 main/端口/连接池。
- 数据必须落 MySQL（重启可查），禁止内存态冒充持久化。
- 测试钩子（`/api/test/*`、`/api/pay/channel-mode`、`/api/jobs/close-expired`、`/api/users`）受 `shop.demo-mode` 门禁：生产 403，演示可用。
