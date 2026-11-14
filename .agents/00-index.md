# 00 · 项目约束（合并版）

> 原 `01-stack-structure`~`09-bugfix` 九个文件已合并到本文件，按主题分节，会话按需跳读。
> 编号仅用于组织内容，所有条款都是本项目实际约束；与 `AGENTS.md` 冲突时以 `AGENTS.md` 为准。

## 1. 技术栈与目录边界

| 项 | 值 |
|---|---|
| 模块 | `shop`（Go module；原 GVA 模板路径已于 2026-11-14 全文替换为 `shop`，源码/配置/文档不再出现旧名） |
| 框架 | gin-vue-admin v2.9.2（Go + Gin + GORM） |
| Go | 1.26.x |
| 数据库 | MySQL 8（3307；业务库 `shop`，框架库 `gva`） |
| 服务端口 | 8099（config `addr`，限定 `shop` 段） |
| 商城页面 | `server/shop/web/`（后端直出单页，非 Vue 构建链） |

`server/shop/` 分层：`api` 绑定/身份/错误码；`service` 事务、状态机、库存守恒、渠道、审计；`model` 表与 DTO（金额禁浮点）；`middleware` JWT 与角色门禁（只信 token）；`initialize` 装配/迁移/种子（幂等，失败即启动失败）；`db` 连接构建（切断 initialize↔service 导入环）；`migrations` 建表种子（已发布脚本不回改）；`router` 公开/登录/角色分组与契约兼容；`web` 内置演示页（5 标签）；`utils` 单号/HMAC/bcrypt（密钥不入库、不打日志）。

改动边界：只允许动 `server/initialize/router_biz.go`（装配）与 config 的 `shop` 段；不改 GVA `system` 模块；不另起 main/端口/连接池；数据必须落 MySQL（禁止内存态冒充持久化）；测试钩子（`/api/test/*`、`/api/pay/channel-mode`、`/api/jobs/close-expired`、`/api/users`）受 `shop.demo-mode` 门禁，生产一律 403。

## 2. 数据与状态机

```
订单：pending_payment → paid → shipped
        │  ↑(审核拒绝)
        ├→ cancelled / closed(超时)
        └→ refund_requested → refunded(渠道成功) / refund_failed(渠道失败,可重试)
```

跃迁只写在 `service/order.go`、`payment.go`、`refund.go`、`shipment.go`；完成判据必须可观察（状态、数字、记录、错误码），不接受"已实现"。

| BR | 要点 | 落点 |
|---|---|---|
| BR-01/02 | 条件 UPDATE 扣减 + 快照 + 占用流水 | `order.go Create` |
| BR-03 | 10 分钟超时（服务端时间）、释放幂等 | `Cancel/CloseExpired` |
| BR-04~06 | HMAC 验签 → 金额 → 幂等（唯一键），拒绝可查 | `payment.go` |
| BR-07 | 有效付款推进 paid；迟到转 `abnormal_payments` | `payment.go Notify` |
| BR-08~10 | 退款申请互斥发货；渠道确认才 refunded；迟到退款不恢复库存 | `refund.go` |
| BR-11 | 发货唯一（`shipments` 唯一键） | `shipment.go Ship` |
| BR-12 | 核查汇总=明细之和；异常无"改单消失"通路 | `report.go` |

异常矩阵 E-01~12（并发争抢、取消/超时竞争、重复通知、验签失败、迟到支付、退款×发货竞争、渠道失败重试、已发货退款、越权、重复发货、改单消除异常）均有明确拒绝/幂等/互斥行为。完整判据见 `shop_word/references/business-rules.md`（只读）。

不变量（每次改动复核）：INV-1 初始可售 = 可售 + 各态占用 + 已出库；INV-2 同一渠道交易号至多归属一单；INV-3 每单至多一次成功发货；INV-4 每单至多一次成功退款。

三条固定防御（审计 I-18 整改后固化）：
1. **渠道调用闸门**：`Retry`/`Audit(approve)`/`HandleAbnormal` 事务内 `SELECT ... FOR UPDATE` 行锁 + 入口状态校验后才调渠道；后到者被闸门拒绝（`ErrRefundNotRetryable`）。真实渠道改闸门状态机 + 异步回调。
2. **错误分类**：仅唯一键冲突（errno 1062，`isDuplicateEntry`）判"重复"，其它 DB 错误原样上抛。
3. **RowsAffected 校验**：条件更新推进状态必须校验受影响行数，不为 1 即回滚。

## 3. 实现模式（写 service 必读）

- **状态跃迁**：`UPDATE ... WHERE 状态=旧值 [AND 条件]` + `RowsAffected==1`；失败读最新态区分重复/过期/非法。禁止先 SELECT 再 UPDATE；禁止 handler 内改状态。
- **幂等**：幂等键落唯一索引（`payments.channel_txn_id`、`payments.order_no`、`shipments.order_no`），冲突按上条分类。
- **一次性资源闸门**：库存恢复用 `restored` 布尔闸门（`UPDATE ... WHERE restored=false` 且 RowsAffected==1 才释放）；迟到支付退款绝不走库存恢复。
- **审计留痕**：每次跃迁调 `logOp(tx, orderNo, actorID, role, action, detail)`，与业务同事务提交，禁止事后补写。
- **演示开关**：渠道退款结果可切 `success|fail` 并带 `RefundCalls()` 计数；超时可缩短但判断用服务端时间。

反模式（出现即打回）：handler/service 混层写事务；用 panic/忽略错误代替业务码；内存 map 冒充持久化；为凑覆盖率给工具函数写用例而业务分支裸奔。

## 4. 工作流与 Git

1. 读本节 + 需求基线材料，列出本次 BR 与 AC。
2. 提交工作项清单：`T 编号 | BR | 影响文件 | 完成判据（引用 AC）| 依赖`，确认后动工。
3. 每项：实现 → 按 AC 自测 → 留证据 → 下一项；禁止一次写完再补过程。

- 分支 `dev/<题号>`（当前 `dev/01-shop`）；禁止直推 `main`、force-push、跳过钩子。
- 提交格式 `[类型] [版本号] [T编号] 中文描述`；类型仅 `feat/fix/hotfix/opt/docs/test`，一次一种；T 编号取自 `docs/dev_log.md`，不得编造或伪装类型。
- 提交前门禁（缺一不可）：`gofmt -l shop` 空输出、`go vet ./shop/...`、`go build ./...`、`go test ./shop/... -count=1`；并发路径追加 `go test -race ./shop/service/`。
- 证据随交：变更文件清单（挂 BR）、自测命令+真实输出+退出码、覆盖率单命令 profile、缺陷记录写入 `docs/review_records.md`。未执行的命令不得写"已验证"。

## 5. 配置与部署

| 项 | 值 | 说明 |
|---|---|---|
| `addr` | ":8099" | 监听端口 |
| `demo-mode` | true | 演示钩子可用；生产必须 false |
| `mysql` | 3307 / `shop` | 业务库连接 |
| `pay-notify-secret` | 演示密钥 | HMAC 验签，不入代码、不打日志 |

坑：配置初始化会重写 config.yaml，其它段也会出现 `addr:`——解析端口必须限定 `shop` 段（见 `scripts/run_acceptance.py read_base_url`）。

CI `.github/workflows/shop-ci.yml`（lint/build/test/coverage）；容器 `deploy/shop/`（Dockerfile + compose + 冒烟）；未真实部署前 DELIVERY/README 保持"未部署"标注。

本地运行：`cd server; go build -o bin\shop-server.exe .; .\bin\shop-server.exe`（8099）；验收 `py -3 shop\scripts\run_acceptance.py --no-build --no-start`。演示账号 `ops1/wh1/cs1/buyer1~3`，口令 `shop123`，商城页 `http://127.0.0.1:8099/shop`。

## 6. 质量与安全

- 自编代码整体语句覆盖 ≥85%，核心包 `service/api/middleware/config/initialize/router` 各 ≥85%。
- profile 必须单命令生成（禁止拼接）：`go test -p 1 ./shop/... "-coverprofile=coverage-single.out" "-coverpkg=./shop/..." -covermode=atomic -count=1`。
- 共库测试一律 `-p 1` 串行；覆盖不足只能补真实业务路径，工具函数凑数视为低价值（审计 I-23）。
- 基线（2026-11-13 拆分后）：并集 88.7%，service 85.5%，api 81.3%，adminapi 81.4%。
- 单元 + HTTP 集成并存，覆盖正常/边界/非法状态/重复/权限/并发（20 并发、退款×发货、重复通知、并发重试）；并发路径必须 `-race`（本地无 cgo 时如实标注）；router 测试不得只断言状态码，库存与金额要有对账断言（I-19）。
- 买家区 6 路由：路由层 `RequireRole(RoleBuyer)` **且**委派 handler 内复核（AuthGuard 委派先于组门禁，仅路由层拦不住）。
- 红线：不编造执行结果/覆盖率/部署记录；不删失败测试、不弱化断言；密钥令牌个人信息不进代码日志；不扩大范围（购物车、多商户、真实物流/支付）；模拟渠道与框架能力不得包装成自研。

## 7. 自检与交接

提交前逐项打勾：gofmt 空、vet/build/test 退出 0；跃迁全走"条件 UPDATE + RowsAffected"；并发路径有闸门与计数断言；DB 错误仅唯一键判重复；覆盖率单命令复跑不低于基线；`demo-mode` 门禁仍在；提交编号在 dev_log 存在且类型匹配；`docs/events.md` 已登记；INV-1~4 有数字。

交接固定七段：工作项（T + BR/AC）→ 变更（文件/接口/表、框架边界）→ 自测（命令+输出+退出码+覆盖率）→ 判据（按 AC 对照，未过写原因）→ 不变量数字 → 风险与待确认（假设编号）→ 下一步。

## 8. 冲突与例外

- 需求材料只读；发现矛盾停下来提请学生裁决并记入 `docs/assumptions.md`，不自行改需求，未确认的假设不落地。
- 优先级：`AGENTS.md` > 本文件 > `versions/` 历史版本；旧提交格式条款与本文件不一致时按本文件执行并在假设表标注待修订。
- 只读对象：`.agents/versions/`（新增 vN + 更新 README 索引，不覆盖旧版）、`shop_word/archive/`（旧交付）、检查者脚本（`shop-trade-check/scripts/acceptance_check.py` 等**永不修改**，包装脚本只改环境）、审计报告、GVA `system` 模块、`C:\Users\Lenovo\Desktop\shop_word` 同名旧副本。
- Windows GBK 控制台：Python 入口强制 `sys.stdout.reconfigure(encoding="utf-8")`，子进程注入 `PYTHONIOENCODING=utf-8`，不要求调用方手动设环境变量（I-24）。

## 9. 缺陷修复

登记（`docs/review_records.md`：问题/复现/期望实际/违反的 BR/版本哈希）→ 先写失败回归测试并跑出真实失败 → 根因定位到行级（`log/zap-error.log`、慢查、并发时序）→ 最小改动修正（不顺手重构）→ 复测（回归转绿 + `go test ./shop/... -count=1` + 受影响验收套件，exit 0）→ 回填留痕并登记 `docs/events.md` → 提交 `[fix][版本][T#]`（引用审计编号如 I-18）。

教训库（已固化）：断言写业务语义不写魔法数字（清扫器）；委派 handler 必须复核角色（契约绕过组门禁）；覆盖率只用单命令（拼接被判不合规）；验收脚本端口解析限定 `shop` 段；渠道重试用事务行锁闸门 + 调用计数；仅唯一键冲突判重复支付；入口强制 UTF-8。

红线：修不了的如实标注"未解决+原因"；不为让测试变绿改测试语义。
