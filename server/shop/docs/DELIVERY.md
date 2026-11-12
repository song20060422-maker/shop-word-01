---
AIGC:
    Label: "1"
    ContentProducer: 001191440300708461136T1XGW3
    ProduceID: b2aa3b4cd48b832ce0392d027319fcdd_9f23f132de4c11f1997d52540024e231
    ReservedCode1: soAGT6YknfC1WBmt8AzvBhDecAqIzDEXYsF0vnF5o1UhC2jmQDamFdg4P0JqvF9lyhefqX0zpgRMKMI2Hbit7Ogl3DgqLtSGbMQQWvWTrNJBbtxTPCdOdOHDhjqQM4s5w7GMZ+cGtDHbO7wbayeTvtt09pv4v1RZLaG+yrgh8sKCMQZcZm7ITYn92AI=
    ContentPropagator: 001191440300708461136T1XGW3
    PropagateID: b2aa3b4cd48b832ce0392d027319fcdd_9f23f132de4c11f1997d52540024e231
    ReservedCode2: soAGT6YknfC1WBmt8AzvBhDecAqIzDEXYsF0vnF5o1UhC2jmQDamFdg4P0JqvF9lyhefqX0zpgRMKMI2Hbit7Ogl3DgqLtSGbMQQWvWTrNJBbtxTPCdOdOHDhjqQM4s5w7GMZ+cGtDHbO7wbayeTvtt09pv4v1RZLaG+yrgh8sKCMQZcZm7ITYn92AI=
---

# Shop 商城小型交易系统 · 交付说明

> 载体：`gin-vue-admin`，业务代码全部位于 `server/shop/`（独立子模块，不侵入 GVA 原有登录/Casbin 体系）。
> 技术栈：Go + Gin + GORM + MySQL 8 InnoDB；框架分层 model / service / api / router / middleware / initialize / migrations / web。

## 1. 启动与入口

| 项目 | 内容 |
| --- | --- |
| 启动 | `cd server && go run .`（默认端口 8099） |
| 业务页面 | `http://127.0.0.1:8099/shop`（单页应用，5 个标签页：买家 / 模拟支付渠道 / 仓库 / 客服 / 运营） |
| 健康检查 | `GET /api/health` |
| 数据库 | `config.yaml` 中 `shop.mysql-dsn`，服务启动时自动执行 `migrations/001_init.sql`（表不存在才建）与 `002_seed.sql`（种子数据幂等写入） |
| 演示账号 | 运营 `ops1`、仓库 `wh1`、客服 `cs1`、买家 `buyer1~3`，口令 `shop123`（`shop.demo-password`） |
| 演示数据重置 | `POST /api/demo/reset`（仅 `shop.demo-mode: true` 时开放） |

预置商品：`SKU-A` 单价 10000 分、可售 5 件；`SKU-B` 单价 5000 分、可售 3 件。

## 2. 数据模型（`migrations/001_init.sql`，11 张表）

`users` 账号与角色 · `sessions` 登录态 · `products` 商品与可售数量 · `orders` 订单（含商品快照、金额、失效时间） ·
`payments` 支付记录（`order_no`、渠道交易号双唯一键）· `payment_notices` 支付通知原始留痕 ·
`refunds` 退款单（`order_no` 唯一，整单退款）· `shipments` 发货登记（`order_no` 唯一）·
`abnormal_payments` 迟到支付待处理 · `inventory_ledger` 库存流水（占用量守恒依据）· `operation_logs` 操作留痕（只增不改）。

## 3. 需求条目 → 实现落点

| 需求 | 实现要点 |
| --- | --- |
| BR-01 在售且数量足够才可下单，订单保存商品与价格快照 | 下单时校验 `on_sale` 与 `available >= qty`，`orders` 落单价/名称快照；后续调价、下架不影响已有订单金额 |
| BR-02 下单即占库存 | 条件更新 `UPDATE products SET available = available - qty WHERE id=? AND available >= qty`，受影响行数为 0 即视为库存不足，天然防超卖 |
| BR-03 待付款 10 分钟、超时按服务端时间关闭 | `orders.expire_at = now + order-timeout-minutes`；`CloseExpired()` 用 `WHERE status=WAIT_PAY AND expire_at<=now` 条件更新，行数=1 才释放，重复关闭不多释放 |
| BR-04 有效支付通知需鉴权、匹配订单与金额 | HMAC-SHA256 验签 → 订单存在 → 金额比对 → 订单处于待付款且在有效期内；任一不满足即拒收并落 `payment_notices`（含失败原因），运营/客服可查 |
| BR-05 重复通知不重复记款、同渠道交易号唯一 | 事务内先以 `WHERE status=WAIT_PAY` 推进订单为已付款（行数≠1 即回滚），再插入 `payments`；`order_no`、`channel_txn_no` 双唯一键保证重复通知/同号归属两单均被数据库否决 |
| BR-06 迟到支付不恢复订单、不重新占库存 | 订单已关闭/取消时，通知写入 `abnormal_payments` 并保留渠道流水，不进主链路、不动库存 |
| BR-07 售前退款：整单、申请期暂停发货 | 仅 `PAID/TO_SHIP` 可申请；申请落 `refunds` 并把订单推进为 `REFUND_PENDING`；发货接口要求订单为 `TO_SHIP`，条件更新失败即拒绝 |
| BR-08 客服同意/拒绝并写原因；拒绝后可继续发货 | 同意→`REFUNDING` 走渠道退款；拒绝→回到 `TO_SHIP` 并记录原因；并发时只有一个条件更新成功，只形成一种合法结果 |
| BR-09 退款渠道成功才恢复库存，且恰好一次 | 退款单 `restored` 闸门（`UPDATE ... WHERE restored=0`）保证只恢复一次；渠道失败保留待处理记录，可重试 |
| BR-10 迟到支付退���不得再次恢复库存 | 异常支付退款分支不调用库存归还，只记账与留痕 |
| BR-11 仓库只能为合法待发货单登记一次发货 | `shipments.order_no` 唯一键 + 订单状态条件更新双保险 |
| BR-12 运营可按订单核查收款/退款/发货/异常，异常不能靠改状态消失 | `/api/ops/orders/:orderNo` 关联查询四类记录；`operation_logs`、`payment_notices`、`abnormal_payments` 只增不改 |

库存归还统一走 `releaseStock()`：以**该订单库存流水净额**判定是否仍有占用，净额不足以归还时直接回滚（错误 2009），既支持运营补货（可售数量可高于初始值），也不会因重复释放虚增库存。

## 4. 验收

验收脚本：`shop-trade-check/scripts/acceptance_check.py`（业务约束 51 项，覆盖 BR-01~12 与 S1~S5 五类场景）。
一键运行（每个场景前自动重置演示数据，避免库存累计消耗影响后续场景）：

```powershell
cd D:\Users\Lenovo\Desktop\shop_word\gin-vue-admin\server\shop
python scripts\run_acceptance.py                 # 编译 + 启动 + 重置 + 五个套件
python scripts\run_acceptance.py --no-build --no-start   # 服务已在运行
```

最近一次结果：

| 套件 | 场景 | 结果 |
| --- | --- | --- |
| normal | 活动正常成交 | 11/11 |
| stock | 缺货与弃单（20 并发抢 5 件） | 7/7 |
| refund | 售前退款（首次失败→重试成功） | 10/10 |
| late | 迟到支付 → 客服退款 | 8/8 |
| races | 操作竞争（并发/重复/越权） | 15/15 |
| **合计** | | **51/51 全部通过** |

> 注：验收脚本各套件按「相对当前可售数量 +1」准备库存，若把五个套件**不作重置**连续执行，可售数量会被合法消耗至不足，`races` 场景下单会因库存不足失败（脚本未捕获该失败分支）。一键脚本在每个套件前调用 `POST /api/demo/reset`，与需求「固定商品、可售 5 件」的演示现场一致。

## 4.1 质量门禁与自动化测试（2026-11-12 补齐）

| 项 | 结果 | 复现命令（`server/` 下） |
| --- | --- | --- |
| gofmt（LF） | 通过（`gofmt -l shop` 空输出） | `gofmt -l shop` |
| go vet | 通过 | `go vet ./shop/...` |
| 全量编译 | 通过 | `go build ./...` |
| 单元 + 集成测试 | 全部通过 | `go test ./shop/... -count=1` |
| 覆盖率 | 总 **87.0%**；service 90.5 / api 88.6 / router 98.4 / middleware 100 / config 92.4 / initialize 85.7 | `go test ./shop/... -coverprofile=coverage.out -coverpkg=./shop/...` |
| 业务验收 | 51/51（2026-11-12 实测） | `py -3 scripts/run_acceptance.py --no-build --no-start` |
| CI | `.github/workflows/shop-ci.yml`：gofmt/vet/build/test/覆盖率阈值/race/冒烟 | 推送触发 |
| 部署 | `server/Dockerfile.shop` + `deploy/shop/docker-compose.yaml`（详见 `deploy/shop/README.md`） | `docker compose -f deploy/shop/docker-compose.yaml up -d --build` |
| race | 本机 Windows 无 cgo 工具链未执行，由 CI（ubuntu）执行 | CI 日志 |

测试文件分布：

| 文件 | 覆盖内容 |
| --- | --- |
| `service/setup_test.go` | 测试库连接、迁移、种子、纯内存模拟时钟 |
| `service/business_test.go` | 状态机、并发下单、重复通知、迟到支付、退款、发货、报表 |
| `service/helpers_test.go` | 错误映射、状态文案、异常退款查询分支 |
| `router/router_test.go` | 真实 HTTP 全链路：登录/下单/支付/发货/退款/迟到/越权/演示重置 |
| `api/api_test.go` | 被契约层遮蔽的原型 handler 直测 |
| `api/branch_test.go` | 参数校验、未知实体、外部渠道、视图映射分支 |
| `config/`、`middleware/`、`db/`、`initialize/`、`utils/` | 配置解析、JWT/角色、自动建库与迁移、Boot 装配、口令/签名/单号 |

**本轮修复的越权问题**：契约委派路径绕过路由组角色门禁，买家可读 `GET /api/ops/summary` 与 `GET /api/cs/refunds`；已在 handler 内复核角色（本轮集成测试发现，非原审计项）。

交付证据索引：`docs/evidence/README.md`（D1–D10）。

## 5. 关键设计取舍

1. **状态机只走条件更新**：所有状态跃迁都是 `UPDATE ... WHERE status = <期望前态>` 并校验 `RowsAffected == 1`，不比对后写，避免并发下双写。
2. **幂等由数据库兜底**：支付通知幂等靠 `payments` 双唯一键，退款靠 `refunds.order_no` 唯一，发货靠 `shipments.order_no` 唯一，而非应用层锁。
3. **库存守恒可审计**：每次可售数量变动都落 `inventory_ledger`（类型 `occupy/release/restore/ship_out` + 变动后数量），运营核查时可用流水复算。
4. **异常只增不删**：迟到支付、金额不符、验签失败均保留原始通知记录，客服处理后追加处理记录，订单状态不会被"改成正常"。
5. **与 GVA 解耦**：业务配置独立于 `shop.*` 配置段，路由在 `initialize/router_biz.go` 扩展点挂载，既有 GVA 登录/Casbin 不受影响。
*（内容由AI生成，仅供参考）*
