# 证据索引（check_evidence.py 40 项核对 → 真实产物映射）

> 生成：2026-11-12 batch2 整改后。本文件只做**指针**，不复制数据；所有证据均为真实执行产物。
> 机器判定：`py -3 shop-trade-acceptance/scripts/check_evidence.py gin-vue-admin --evidence-dir gin-vue-admin/docs/evidence`
> 人工核定项（E2 场景证据齐全、E4 抽查一致性）见文末。

## 自动判定项映射

| 核对项 | 关键词命中 | 真实证据 |
|---|---|---|
| R1-1 在售与库存校验下单 | 在售、缺货、stock | 验收日志 R1.1/R1.4（51/51 全 PASS，`acceptance-20261112-fix2.log`）；`service/business_test.go TestCreateOrderGuards` |
| R1-2 价格快照 | 价格快照、改价 | 验收日志 R1.2/R1.2b（下单金额=单价×数量；调价/下架不改旧单金额） |
| R1-3 占用与可售 | 占用、可售、available | 验收日志 R1.3/R2.2/R2.2c；`inventory_ledger` 流水对账（`gate_test.go` 库存恢复恰 1 条断言） |
| R1-4 数量限制与单位分 | 件数、数量限制、quantity | 验收日志 R1.4 20 并发争抢 5 件恰好 5 成功；金额全链路单位分（`model` 金额字段 int64 cents） |
| R2-1 10 分钟超时 | 超时、timeout、expire | 验收日志 R2.1/R2.1b；服务端时间判定（`order.go CloseExpired`），`router_test.go TestLatePaymentAbnormal` |
| R2-2 取消与重复取消幂等 | 重复取消、释放 | 验收日志 R2.2/R2.2b/R2.2c（重复取消被拒且不多释放） |
| R2-3 释放后可再购 | 释放、再购买 | 验收日志 R2.3 |
| R3-1 通知验签 | 验签、签名、sign | 验收日志 R3.1/R3.4c（验签失败拒绝且可查）；`payment.go` HMAC-SHA256，`utils.PaySign` |
| R3-2 金额不符/未知订单 | 金额不符、未知订单 | 验收日志 R3.4/R3.4b（拒绝且 `payment_notices` 留痕可查） |
| R3-3 重复通知幂等 | 重复通知、幂等、duplicate | 验收日志 R3.2/R3.2b（并发重复通知只记款一次）；`payments` 唯一键 + `isDuplicateEntry` 错误分类 |
| R3-4 渠道交易号唯一 | 交易号、transaction | 验收日志 R3.3（同一交易号不能归属第二个订单） |
| R4-1 有效付款进待发货 | 待发货、paid | 验收日志 R4.1（记款后订单待发货） |
| R4-2 迟到支付不复活订单 | 迟到支付、异常支付、late payment | 验收日志 R4.2/R4.2b（不恢复订单、不重占库存）；`payment.go markAbnormal` |
| R4-3 异常列表与处理 | 异常列表、异常处理 | 验收日志 R4.2c/R4.3（客服异常待处理列表+发起退款）；`AbnormalList/HandleAbnormal` |
| R5-1 退款申请暂停发货 | 申请退款、暂停发货、refund request | 验收日志 R5.1/R5.2（申请期间发货被拒）；`refund.go Apply` 置 refund_requested |
| R5-2 客服审核同意/拒绝 | 批准、拒绝、审核 | 验收日志 R5.3/R5.3b（拒绝必填原因、拒绝后可继续发货）；`Audit(approve bool, reason)` |
| R5-3 拒绝与重新申请 | 拒绝、回到待发货 | 验收日志 R5.3b；`compat_service_test.go TestApplyRefundGuards`（拒绝后复用同一退款单重新申请） |
| R5-4 退款×发货竞争 | 并发、竞争 | 验收日志 R5.4/R5.4b（并发只产生一种合法结果）；`service/gate_test.go` 并发闸门测试 |
| R6-1 渠道失败保留待处理 | 渠道退款失败、重试 | 验收日志 R6.1/R6.1b/R6.1c（失败留记录→重试成功）；`refund.go Retry` + 行锁闸门 |
| R6-2 退款结果与库存恢复 | 退款结果、恢复 | 验收日志 R6.2/R6.2b（买家可见；售前退款恢复恰好一次） |
| R6-3 迟到退款不恢复库存 | 不恢复库存、未重复释放 | 验收日志 R6.3/R6.3b（库存不变、金额计入汇总恰一次）；`HandleAbnormal` 绝不调 releaseStock |
| R6-4 重复退款拒绝 | 重复退款、拒绝 | 验收日志 R6.4；INV-4（每单至多一次成功退款） |
| R7-1 发货唯一 | 发货、运单 | 验收日志 R7.1/R7.1b（二次发货被拒）；`shipments.order_no` 唯一键 |
| R7-2 运单查询 | 运单、承运方 | 验收日志 R8.1（买家可查询运单信息）；`shipment.go Get/List` |
| R7-3 角色越权拒绝 | 越权、403、无权 | 验收日志 R7.3/R7.3b/R8.4（买家不能发货、仓库不能审核、不能看他人订单）；`router/security_test.go TestBuyerRoleGate` |
| R8-1 运营核查汇总 | 汇总、收款、已退款 | 验收日志 R8.2/R8.2b（收款/退款汇总含本单）；`report.go Summary` |
| R8-2 明细可查 | 记录、轨迹、异常记录 | `report.go OrderDetail/Trail/Ledger`（收款、退款、发货、异常记录与轨迹）；验收日志 R8.1 |
| R8-3 异常不可改单消除 | 异常记录、operation_logs | `operation_logs` 全跃迁留痕；异常只改自身状态（`AbnormalHandled`），无改订单状态通路 |
| S1~S5 五组场景 | 场景 | 五套件 51/51（`acceptance-20261112-fix2.log`：normal 11/11、stock 7/7、refund 10/10、late 8/8、races 15/15） |
| E1 并发争抢 | 并发下单 | 验收日志 R1.4；`races` 套件 |
| E2 场景证据齐全 | （人工核定） | 见文末 |
| E3 测试证据可追溯 | 测试记录 | `docs/evidence/`（覆盖率 profile、验收日志×2、安全回归、gofmt/vet 记录） |
| E4 抽查一致性 | （人工核定） | 见文末 |
| D1 交付清单 | 交付 | `server/shop/docs/DELIVERY.md`（D1~D10 随交清单） |
| D2 迁移种子 | 迁移、seed | `server/shop/migrations/*.sql`（幂等脚本，启动即建库灌数据） |
| D3 部署说明 | 部署 | `deploy/shop/`（Dockerfile+compose+冒烟）；**未实际部署上线**（如实标注，见 DELIVERY §部署） |
| B1 会话记录 | 会话 | `docs/events.md`（23 号事件流水）、`docs/prompt_export.md`、`提示词/` 15 文件 |
| B2 Skill 使用 | skill | `docs/skill-usage.md`（shop-trade-check 多次调用 + shop-trade-acceptance 本报告即首次调用产物） |
| B3 AI 使用说明 | AI | `docs/ai-usage.md`、`docs/prompt_export.md` |
| B4 JSONL 导出 | jsonl | **缺失**：IDE 会话 JSONL 未导出（`docs/evidence/sessions/` 不存在）——如实标注待补，导出后补录 |
| B5 约束文件 | agents | `.agents/`（00~09 编号文件 + versions v1~v3 + README 加载记录含实际加载行） |
| B6 提交规范 | commit | `git log` 全部符合 `[类型][v0.2.0][T#]`；`AGENTS.md`/`.agents/03-workflow-git.md` |

## 人工核定项

- **E2 场景证据齐全**：五组场景在 51 项验收日志逐条 PASS（见 S1~S5 行），并发场景另由 `gate_test.go` 渠道调用计数断言佐证 → 核定通过；讲解视频尚未录制（依赖出镜），缺失项已如实标注。
- **E4 抽查一致性**：抽查 3 条（R3.2 重复通知→`payments` 计数=1；R6.2b 库存恢复→`inventory_ledger` 恰 1 条 restore 流水；R6.3 迟到退款→available 与退款前一致）均与数据库实况一致。

## 结论

自动判定 43 项（含本索引后）→ 缺口仅 **B4（JSONL 导出）**，为环境待办而非实现缺陷；其余全部有真实产物支撑。

## 2026-11-13 增补（batch5 整改会话，提交 8b0d468~2ddb551）

| 编号 | 类型 | 产物/位置 |
| --- | --- | --- |
| N1 | 覆盖率（分包口径 82.8%） | docs/evidence/coverage-20261113.out（go tool cover -func 总计行可复核） |
| N2 | 覆盖率（跨包并集 88.7%，标准单命令） | docs/evidence/coverage-single-20261113.out |
| N3 | 新增 adminapi 测试 | server/shop/adminapi/adminapi_test.go（adminapi 0→81.4%） |
| N4 | 新增 api 契约全链路 HTTP 测试 | server/shop/api/contract_http_test.go（api 54.1%→81.3%） |
| N5 | 管理端前端构建（I-27） | docs/evidence/frontend-build-20261113.log（web/dist 三个页面 chunk 名与字节数） |
| N6 | 双服务镜像重建与部署（I-25） | docs/evidence/deploy-dual-20261113.md（shop-buyer/shop-admin:8b0d468 + admin-seed + 容器冒烟） |
| N7 | 业务验收复跑（I-28） | docs/evidence/acceptance-20261113-newarch.log（51/51，:8090 独立买家端） |
| N8 | 独立测试报告（I-28） | docs/evidence/independent-test-20261113.md（覆盖 8b0d468 新架构） |
| N9 | CI run #14（**success**） | push 2ddb551 触发，https://github.com/song20060422-maker/shop-word-01/actions/runs/43355394482 ；步骤明细 `docs/evidence/ci-run14.md` + `ci-run14-jobs.json`（含双服务冒烟与覆盖率阈值步骤） |
| N10 | 事件登记 | docs/events.md #34~#40；docs/dev_log.md §4 提交清单增补 8b0d468/5e4529b/2ddb551/本提交 |
| N11 | CI run #15（**success**） | push 7e3197c（docs 提交）触发，同上工作流全部步骤通过 |
| N12 | 约束入口 AGENTS.md | `AGENTS.md`（新建，未改动 `AGENT.MD`）：会话必读顺序、任务→`00~09` 映射、硬性红线、三角色独立会话、加载留痕、冲突裁决、当前架构事实；`.agents/README.md` 加载记录同步补行 |
| N13 | CI run #16 / #17（均 **success**） | #16 @29b49ff（run #15 留证文档）、#17 @b924756（新增 AGENTS.md 与加载记录） |
| N14 | 后台三页独立化（c9ff4c2/46b124c/852e4ce） | `server/shop/web/ops|warehouse|cs.html` + `/gva/*` 反向代理；镜像 `shop-buyer/shop-admin:852e4ce` 容器冒烟（18099 四页面 200、/gva 登录与 ops/warehouse/cs 三接口 code=0） |
| N15 | CI run #19（**success**） | push 852e4ce 触发，全部步骤通过 |
| N16 | CI run #20（**success**） | push 2d7cd0e（事件/提交清单/证据索引登记）触发，全部步骤通过；dev/01-shop HEAD=2d7cd0e |
| N17 | Skill 业务验收（shop-trade-check） | `docs/evidence/skill-check-20261113.log`：五组场景 **51/51 通过**，退出码 0（配置 `server/shop/docs/acceptance-skill-config.json`） |
| N18 | Skill 交付验收（shop-trade-acceptance） | `docs/evidence/acceptance-report-skill-20261113.md`：业务规则 32 条逐项 PASS、S1–S5 全通过、证据核对 **43/43 无缺失** |
| N19 | 覆盖率（跨包并集 88.4%） | `docs/evidence/coverage-skill-20261113.out`（标准单命令 `-p 1 -coverpkg=./shop/...`，`go tool cover -func` total 行可复核） |
| N20 | 本轮缺口补全 | 数量越界拒绝（1004）、客服处理强制原因（1005）、`router/pages_test.go` 覆盖四页面与 `/gva` 反代；单测 `branch_test.go` / `business_test.go` 同步加固 |
| N21 | CI run #21（**success**） | push 96f35bb（fix 8bc0e1d + test 84e3171 + docs）触发，全部步骤通过 |
| N22 | 容器重建 7309737 冒烟 | `shop-buyer`(18099)/`shop-admin`(18098) 均 healthy；18099 四页面 200 且 `/cs` 含"必须填写审核原因"、`quantity=0`→409、空原因审核→`code=7 客服处理必须填写原因`、带原因审核 approved+success、`ops/summary` 收款 10000/退款 10000 对账一致；UTF-8 请求体中文原因落库正常 |
| N23 | CI run #23/#24（均 **success**） | #23 @7309737（客服审核交互修复）、#24 @330923a（容器重建留证） |
| N24 | 集成版全功能页恢复（86e3144） | `/` 与 `/index.html` 输出五页签集成页；独立页 `/shop` `/ops` `/warehouse` `/cs` 保留；镜像 `86e3144` 容器冒烟：五入口 200、根路径含五页签、容器内 products/ops.summary/orders/ledger 均 200 |
| N25 | 支付通知双形态验签（8eee633） | 演示页形态（`channelTxnId` + body `sign`）→ `code=0 status=SUCCESS`、订单 `paid`、错签 400；契约形态（snake_case + `X-Channel-Signature`）回归 `code=0`；`contract_http_test` 新增两用例；容器 `8eee633` 复验通过 |
| N26 | 后台三页白底 + 角色分权（a47947a） | 本地 8099 与容器 18099：ops1/wh1/cs1 登录 `code=0`、`authorityId` 分别 9001/9002/9003；ops1 调 `/shop/ops/summary`→code=0、`/shop/cs/refunds`→code=7；cs1 反之；`/shop` `/ops` `/warehouse` `/cs` 四页均白底（`#f4f6fb`+蓝色页签）；镜像 `shop-admin/shop-buyer:a47947a` healthy |

