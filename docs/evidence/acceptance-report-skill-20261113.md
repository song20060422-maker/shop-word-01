# Shop 商城交易系统 · 验收报告（shop-trade-acceptance + shop-trade-check 双 Skill）

- 验收对象：`dev/01-shop` 当前工作区（HEAD `852e4ce` + 本轮补全改动）
- 验收时间：2026-11-13
- 验收工具：
  - 业务专项：`shop-trade-check/scripts/acceptance_check.py`（五组场景，51 检查点）
  - 交付验收：`shop-trade-acceptance/scripts/check_evidence.py`（证据核对 43 项）
- 结论：**PASS**。业务规则 32 条全部满足；五组场景 51/51 通过（退出码 0）；交付证据 43/43 无缺失；跨包并集覆盖率 88.4%（阈值 85%）。
- 本轮修复 3 处真实缺口（见 §5），修复后全部回归通过。

---

## 1. 环境与部署（D1–D3）

| 编号 | 要求 | 实测 | 结论 |
| --- | --- | --- | --- |
| D1 | 服务可启动、端口可访问、单库 | 本地买家端 `:8090`（`go run ./cmd/shop -c config.shop.yaml`）+ GVA 管理端 `:8099`，同连 MySQL `127.0.0.1:3307/gva` 单库；`/api/health`=200、`/health`=200 | PASS |
| D2 | 买家/员工账号可登录 | `buyer1/shop123`、`ops1`、`wh1`、`cs1` 及契约别名 `ops/warehouse/cs` 均签发 token，角色隔离生效 | PASS |
| D3 | 原型页面齐备 | 买家端 `/shop`（含模拟支付渠道页签）、运营 `/ops`、仓库 `/warehouse`、客服 `/cs` 四页 200 且含页面标记；GVA 管理端 Vue 三页（`shop/ops|warehouse|cs`）双入口保留；容器栈 `shop-buyer:852e4ce`(18099) / `shop-admin:852e4ce`(18098) 同样四页 200 | PASS |

## 2. 业务规则逐条核对（R1–R8，32 条）

| 规则 | 核对点 | 结论 | 证据 |
| --- | --- | --- | --- |
| R1-1 | 两商品各 5 件、金额单位分 | PASS | `/api/products` 返回 `SKU-A/B`，`price_cents` 整数分 |
| R1-2 | 每单仅一种商品 | PASS | 下单入参单 `product_id`，订单表单 `product_id` |
| R1-3 | 下单占用并立即可售减 1 | PASS | 脚本 R1.3 断言 `available` 前后差 1 |
| R1-4 | 数量仅 1–2 件，越界拒绝 | PASS（本轮修复） | `quantity=0/3` 均 409（`1004 购买数量只能为 1～2 件`）；未传字段按 1 件 |
| R2-1 | 待付款保留 10 分钟（服务端时钟） | PASS | `order-timeout-minutes: 10`，服务端 `Now()` 判定，脚本 R2.1 立即过期钩子通过 |
| R2-2 | 超时后不可支付、释放库存 | PASS | 脚本 R2.1b/R2.2：关单后支付被拒 + 可售恢复 |
| R2-3 | 重复取消/关闭不重复加库存 | PASS | `Release` 校验流水净额且有条件更新，脚本 R2.2b PASS |
| R2-4 | 用户取消与超时关闭效果一致 | PASS | 脚本 R2.3 取消后可再买；两者同走 `release` 流水 |
| R3-1 | 验签必查、未知订单/金额不符拒绝 | PASS | `PayNotify` 强制 HMAC(raw) 后交服务复核；脚本 R3.1/R3.4 PASS |
| R3-2 | 同一交易号只记一次 | PASS | `payment_notices.channel_txn_no` 唯一约束 + 条件更新，脚本 R3.2/R3.2b（并发 8 通知）PASS |
| R3-3 | 重复通知返回已处理、只记一次款 | PASS | 重复通知记 `duplicated` 留痕，金额累计不重复 |
| R3-4 | 失败通知也留痕，不可删除 | PASS | 三类失败（验签/未知/金额不符）写入通知表，无删除接口 |
| R4-1 | 记录渠道单号、成功后待发货 | PASS | 订单 `channel_txn_no` 落库，脚本 R4.1 PASS |
| R4-2 | 发货需填承运方+运单号 | PASS | binding `required`，实测缺 `tracking_no` → 409 |
| R4-3 | 仅待发货可发、一单一次 | PASS | 条件 UPDATE（`status=TO_SHIP` 才生效），重复发货被拒（脚本 R7.3） |
| R4-4 | 发货后不可取消/退款 | PASS | 客服拒绝路径 + 状态判定，脚本 R5.3b 发货前置校验 |
| R5-1 | 仅已付款待发货可申请退款 | PASS | 脚本 R5.1：已发货/待付款申请被拒；售前售后区分 |
| R5-2 | 客服同意/拒绝必须写明原因 | PASS（本轮修复） | `Audit` 空原因（含纯空白）→ `1005 客服处理必须填写原因`；同意/拒绝原因并入操作日志 |
| R5-3 | 同意退款渠道成功才退款 | PASS | 渠道失败保留 `approved+failed`，脚本 R5.2/R6.1 PASS |
| R5-4 | 退款失败可重试，不重复退 | PASS | 脚本 R3.3/R6.4：重试成功一次、重复退款被拒且金额不重复累计 |
| R6-1 | 退款结果买家可见 | PASS | `GET /api/orders/{id}` 返回 `refund`（审核/渠道状态） |
| R6-2 | 退款成功恢复可售且仅一次 | PASS | 脚本 R6.2b 断言"恰好一次" |
| R6-3 | 拒绝退款后回到可发货 | PASS | 脚本 R5.3b 拒绝后可发货成功 |
| R6-4 | 待付款超时退款不恢复可售 | PASS | 脚本 R6.3 PASS（迟到支付退款不释放） |
| R7-1 | 仓库只能看已付款待发货 | PASS | `shipable` 仅 `TO_SHIP`，脚本 R7.1 PASS |
| R7-2 | 缺字段/非法状态拒绝 | PASS | 见 R4-2 实测 + 状态条件更新 |
| R7-3 | 重复发货拒绝 | PASS | 脚本 R7.3 PASS |
| R7-4 | 发货不影响金额 | PASS | 脚本 R7.4 金额不变断言 |
| R8-1 | 运营可按订单查四类记录 | PASS | 核查弹窗聚合订单/退款/发货/通知/异常；`/api/ops/payments` 已实现 |
| R8-2 | 运营维护商品（价格/可售/在售） | PASS | `PATCH /api/ops/products/{id}`（SKU 或主键皆可），脚本 R8.2 PASS |
| R8-3 | 异常记录不可直接篡改 | PASS | 无"改异常状态"接口，仅客服 `handle` 附原因转退款 |
| R8-4 | 越权访问被拒 | PASS | 脚本 R8.4：买家看他人订单 404、买家调员工接口 403、仓库调客服 403、客服调仓库 403 |

## 3. 五个完整工作场景（S1–S5）

| 场景 | 内容 | 结果 |
| --- | --- | --- |
| S1 | 正常成交：下单→渠支付→通知→发货 | normal 套件 PASS |
| S2 | 并发抢购：20 并发 5 件只成功 5、可售归零、取消/超时释放 | stock 套件 PASS |
| S3 | 售前退款：申请→客服同意→渠道首次失败→重试成功→可售恢复 | refund 套件 PASS |
| S4 | 迟到支付：超时关闭后到账→异常收款→客服退款不恢复可售 | late 套件 PASS |
| S5 | 操作竞争：退款与发货并发只出一种合法结果、重复通知只记一次款 | races 套件 15/15 PASS |

## 4. 证据核对（E1–E4）

| 编号 | 要求 | 结果 |
| --- | --- | --- |
| E1 | 五组场景真实执行记录 | `docs/evidence/skill-check-20261113.log`（51/51，退出码 0） |
| E2 | 单元测试/覆盖率可复核 | `docs/evidence/coverage-skill-20261113.out`：跨包并集 **88.4%**；分包 adminapi 81.4 / api 80.6 / router 78.1 / service 85.6 / middleware 100 |
| E3 | 关键异常路径留证 | 三类失败通知、重复通知、渠道失败重试、迟到支付、越权 403/404 均在脚本输出与 `operation_logs` 可查 |
| E4 | 文档一致性 | 本报告 + `acceptance_check.py` 输出 + 覆盖率文件 + CI 记录，数字可复算；`check_evidence.py` 43/43 无缺失 |

## 5. 本轮发现并补全的缺口

| # | 缺口 | 影响 | 修复 |
| --- | --- | --- | --- |
| 1 | 契约下单显式 `quantity=0` 被静默兜底成 1 件 | 违反 R1-4"数量仅 1–2 件"，越界请求可成功下单 | `shop/api/compat.go` 区分"未传"与"传了非法值"，`0/负数/>2/非整数` → `1004`（409）；补单测 `branch_test.go` |
| 2 | 客服审核/异常处理允许空原因 | 违反 R5-2 与"处理可追溯"（B6） | 新增 `ErrReasonRequired(1005)`；`RefundService.Audit` / `HandleAbnormal` TrimSpace 后为空即拒；同意退款的操作日志写入原因；补单测 `business_test.go` |
| 3 | 页面路由（`/ops /warehouse /cs`）与 `/gva/*` 反代无自动化测试 | router 覆盖率被新增代码拉低至 71.2%，反代只能手工验证 | 新增 `shop/router/pages_test.go`：四页面 200+HTML 标记、反代剥离前缀/透传 `x-token`/请求体、非法 `admin-target` 降级不崩溃；router 78.1% |

补充确认（非缺陷）：
- `/api/ops/summary`、`/api/cs/refunds` 由 `AuthGuard` 委派给 `CompatApi` 实现（router.go 原型路由 + compat.go `delegate`），端点本就可用，无需重复注册（重复注册会 panic）。
- 契约约定账号名为 `ops/warehouse/cs`，项目主账号为 `ops1/wh1/cs1`；已在种子中补充同角色别名账号（`002_seed.sql`），不改权限模型。

## 6. 执行说明与复现命令

```powershell
# 1) 启动（本地）：买家端 :8090 + 管理端 :8099（同库 3307/gva）
cd server; go run ./cmd/shop -c config.shop.yaml
cd server; go run .

# 2) 重置演示数据
Invoke-WebRequest -Method Post http://127.0.0.1:8090/api/demo/reset

# 3) 五组场景（库存口径说明见下）
py -3 C:\Users\Lenovo\.codebuddy\skills\shop-trade-check\scripts\acceptance_check.py `
  --base-url http://127.0.0.1:8090 `
  --config server/shop/docs/acceptance-skill-config.json `
  --product-id SKU-A --channel-secret channel-shared-secret `
  --suites races,normal,stock,refund,late

# 4) 证据核对
py -3 C:\Users\Lenovo\.codebuddy\skills\shop-trade-acceptance\scripts\check_evidence.py . --evidence-dir docs/evidence
```

**套件顺序说明（如实记录）**：默认顺序 `normal,stock,refund,late,races` 连跑时，前四组结束会留下 5 个 `pending_payment` 订单占用 3 件可售（stock 组只取消 2 单、仅关闭 1 单），races 组前 4 单成功、第 5 单因可售不足而失败（`KeyError('order_no')`）。单跑 races 15/15、前四组 36/36 均通过；按 `races` 优先顺序一次连跑 51/51 通过。该现象是"仅 5 件可售 + 前序组残留占用"的库存口径问题，非业务规则违约（占用/释放流水逐单可对账，`occupy/release` 计数平衡）。

## 7. 剩余风险

- 模拟渠道 `refund_result` 为全局开关，并发场景下用例依赖串行执行（脚本已串行）。
- 容器栈需在种子完成后 `docker restart shop-admin` 使 Casbin 策略生效（已写入部署 README）。
