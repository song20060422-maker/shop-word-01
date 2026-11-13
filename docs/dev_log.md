# 工作项清单（dev_log）

> 题目编号：**01 · Shop 商城小型交易系统**（飞书题面《Go项目业务场景题_共同要求》+ 需求文档）
> 代码基线：`gin-vue-admin` v2.9.2，业务全部位于 `server/shop/`
> 提交格式（约束 v3）：`[类型] [版本号] [T编号] 中文描述`
> **编号规则**：提交信息中的 `T编号` 必须在本文件登记，且对应题目 01 的需求编号（BR/E/S/AC/INV）。
> 禁止使用未登记的编号；不允许用内部编号替代题目编号——每条工作项同时标注题目 01 的 BR/AC 归属。

## 1. 编号映射（题目 ↔ 工作项）

| 题目编号 | 工作项编号 | 覆盖需求 |
| --- | --- | --- |
| 01 | T1 | 工程骨架、配置、迁移、种子账号（BR 前置条件、AC-01 数据基线） |
| 01 | T2 | 商品与下单：在售校验、库存原子扣减、金额快照（BR-01/02，AC-01/02） |
| 01 | T3 | 支付通知：验签、金额校验、幂等记款、迟到支付转异常（BR-04/05/06/07，AC-04/05） |
| 01 | T4 | 售前退款：申请、暂停发货、审核、渠道失败重试、库存恢复闸门（BR-07/08/09，AC-06/07） |
| 01 | T5 | 发货：唯一发货、状态门禁（BR-11，AC-07） |
| 01 | T6 | 运营核查与留痕：汇总、轨迹、流水、异常只增不删（BR-12，AC-08/09） |
| 01 | T7 | 内置原型页 5 标签 + 模拟渠道控制台（S-1~S-5 可演示） |
| 01 | T8 | 接口契约兼容层与 51 项验收脚本（S-1~S-5 全场景） |
| 01 | T9 | 测试导入环修复、质量门禁（gofmt/vet/覆盖率/CI） |
| 01 | T10 | 交付证据 D1–D10、分支与提交留痕 |

## 2. 工作项明细

| T | 需求编号 | 影响文件 | 完成判据（AC） | 状态 |
| --- | --- | --- | --- | --- |
| T1 | AC-01 数据基线 | `shop/config/*`、`shop/db/db.go`、`shop/migrations/*`、`shop/initialize/boot.go` | 服务启动自动建库建表 + 种子账号可登录；重复启动幂等 | 已完成 |
| T2 | BR-01/02、AC-01/02 | `shop/service/order.go`、`shop/service/catalog.go`、`shop/model/*` | 下架/超量下单被拒不建单；下单后可售 5→3；调价不影响旧单金额 | 已完成 |
| T3 | BR-04~07、AC-04/05 | `shop/service/payment.go`、`shop/api/compat.go` | 验签失败/金额不符/未知订单被拒且可查；重复通知只记款一次；迟到支付转异常 | 已完成 |
| T4 | BR-07~10、AC-06/07 | `shop/service/refund.go`、`shop/service/channel.go` | 申请期发货被拒；拒绝后可发货；渠道失败可重试；退款恢复库存恰好一次 | 已完成 |
| T5 | BR-11、AC-07 | `shop/service/shipment.go` | 二次发货被拒；已发货订单退款申请被拒 | 已完成 |
| T6 | BR-12、AC-08/09 | `shop/service/report.go`、`shop/service/*` 留痕 | 汇总=明细之和；异常处理后仍可查；无改单消除异常通路 | 已完成 |
| T7 | S-1~S-5 | `shop/web/*`、`shop/router/router.go` | 5 标签页面可操作，无静态假数据 | 已完成 |
| T8 | S-1~S-5、AC 全量 | `shop/api/compat.go`、`shop/router/compat_router.go`、`shop/scripts/run_acceptance.py` | 验收脚本 51/51 通过（2026-11-12 实测，见 `docs/evidence/acceptance-2026-11-12.log`） | 已完成 |
| T9 | 质量门禁 | `shop/db/db.go`（新增）、`shop/*/*_test.go`、`.github/workflows/shop-ci.yml` | `go test ./shop/...` 全通过；覆盖率 87.0%；gofmt/vet 通过；CI 配置就绪 | 已完成 |
| T10 | 交付证据 | `docs/*`、`docs/evidence/*` | D1–D10 材料齐全可检索 | 已完成 |

## 3. 本轮修复记录（T9）

| 问题 | 根因 | 修正 | 复测 |
| --- | --- | --- | --- |
| `go test ./shop/service` 编译失败（导入环） | 测试包 `service` → `initialize` → `service` 循环引用 | 抽出 `shop/db` 包承载连接/迁移/种子，`initialize.Boot` 与测试共同依赖 | `go test ./shop/...` exit 0 |
| 24 个文件 gofmt 不合规（CRLF） | Windows 检出自动 CRLF | 全部转 LF + `gofmt -w`；`.gitattributes` 锁 `*.go text eol=lf` | `gofmt -l shop` 输出为空 |
| 买家可看运营汇总（越权） | 契约委派绕过路由组 `RequireRole` | `OpsSummary`/`CSRefunds` 内复核角色 | 集成测试断言 403 |
| 异常退款重试失败 | `refunds.order_no` 唯一键与重复建单冲突 | `HandleAbnormal` 复用失败退款单重试 | 测试通过 |
| 重复通知返回 nil 错误 | 未把 duplicated 结果映射为哨兵错误 | 事务提交后返回 `ErrDuplicatedPay` | 测试通过 |
| 买家可维护商品 | `CatalogService.Update` 缺角色门禁 | 仅 `ops` 可维护 | 测试通过 |

## 4. 提交清单（本地 dev/01-shop 分支，真实 commit）

| 提交 | 说明 |
| --- | --- |
| `0ec1170` | [fix][v0.2.0][T9] 拆分 shop/db 包修复测试导入环 |
| `03233ed` | [fix][v0.2.0][T9] 修复重复通知语义、异常退款重试、商品权限与契约委派越权 |
| `63f863e` | [test][v0.2.0][T9] 补齐 HTTP 集成与分支测试（覆盖率 66.8%→87.0%） |
| `004f5e9` | [ci][v0.2.0][T9] 新增 CI 质量门禁与 Shop 镜像、compose 部署 |
| `63d2c90` | [chore][v0.2.0][T9] 统一 LF 与 gofmt 格式 |
| `ff0e0e4` | [docs][v0.2.0][T10] 补齐 D1–D10 证据与交付文档 |
| `24f694f` | [chore][v0.2.0][T10] 补齐 gva 框架库初始化种子（管理后台 admin 账号） |
| `0caa980` | [fix][v0.2.0][T9] 修复 Boot 覆盖 global.GVA_DB：Shop 使用独立连接 |
| `abbcea2` | [fix][v0.2.0][T9] 安全整改（审计 I-01/I-02）：demo-mode 门禁 + 买家区角色复核 |
| `d6b8bee` | [test][v0.2.0][T9] 补 service 缺口用例与安全回归用例；修复验收脚本端口解析 |
| `5f4a8eb` | [docs][v0.2.0][T10] 覆盖率证据以标准单命令重生成（二轮整改闭环登记） |
| `d2a520d` | [fix][v0.2.0][T9] 审计 I-18：渠道退款行锁闸门 + 错误分类 + RowsAffected 校验 |
| `a17194a` | [test][v0.2.0][T10] 审计 I-23/I-24：契约兼容层真实业务测试 + 验收链路强制 UTF-8 |
| `3d4b257` | [fix][v0.2.0][T10] 验收包装脚本强制 UTF-8（I-24） |
| `c6be8d7` | [docs][v0.2.0][T10] 约束文件重构（I-07/I-21）：.agents 00~09 编号文件集 |
| `e067a24` | [test][v0.2.0][T10] 自编 Skill 实际调用留痕（I-06）：验收 Skill 43/43 + 业务 Skill 51/51 |
| `8b0d468` | [refactor][v0.2.0][T11] 数据库合一与前后台服务拆分：业务表并入 gva 单库（备份留证），测试统一 shop_test；买家端拆为独立 Gin 服务 server/cmd/shop(:8090)；运营/仓库/客服后台入 GVA 管理端（adminapi + /shop/** Casbin + Vue 三页 + 幂等种子 SQL） |
| `5e4529b` | [test][v0.2.0][T11] 审计 I-26：补 adminapi 测试（0→81.4%）与 api 契约全链路 HTTP 测试（54.1%→81.3%）；覆盖率双口径复测（分包 82.8% / 跨包并集 88.7%） |
| `2ddb551` | [deploy][v0.2.0][T11] 审计 I-25：双服务镜像重建部署——Dockerfile.gva + config.gva-docker.yaml，compose 扩为 shop-buyer(18099)+shop-admin(18098)+admin-seed 种子任务（实测冒烟通过） |
| 本提交 | [docs][v0.2.0][T11] 审计 I-25~I-28 整改闭环：DELIVERY.md/.agents05 覆盖率改真实复算值；npm run build 留证（I-27）；51/51 验收复跑 + 独立测试报告（I-28）；events #34~#40；CI run #14 @2ddb551 结论留证 |
| `c9ff4c2` | [feat][v0.2.0][T11] 后台管理三页独立化：shop/web/ops|warehouse|cs.html（原型深色风）随买家端 :8090 托管，/gva/* 反向代理调 GVA adminapi（JWT+Casbin 不变），GVA Vue 三页保留双入口 |
| `46b124c` | [fix][v0.2.0][T11] 外链菜单点击无反应修复：gva 判外链看菜单 name 而非 path，种子 SQL 改 name+path 同置 URL（幂等） |
| `852e4ce` | [deploy][v0.2.0][T11] 容器配置补 admin-target，镜像重建 shop-buyer/shop-admin:852e4ce 并冒烟（四页面 200 + /gva 反代三接口 code=0），CI run #19 success |
| 本提交 | [fix][v0.2.0][T11] Skill 验收缺口补全：数量越界（显式 0）不再兜底、客服审核/异常处理强制写明原因（1005 + 日志留痕）、补契约账号别名；新增 pages_test 覆盖页面与 /gva 反代；五组场景 51/51、证据 43/43、覆盖率 88.4%，报告见 `docs/evidence/acceptance-report-skill-20261113.md` |
| `7309737` | [fix][v0.2.0][T11] 客服审核交互对齐"必须填写原因"：独立页取消不再被默认原因兜底提交、空白原因前端拦截；GVA 客服页 askReason（非空校验+取消捕获）。镜像重建 `shop-buyer/shop-admin:7309737`，容器冒烟：四页 200、qty=0→409、空原因→1005 文案、带原因审核 approved+success、汇总对账一致 |
| `86e3144` | [feat][v0.2.0][T11] 集成版全功能页恢复挂载到 `/` 与 `/index.html`（买家/渠道/仓库/客服/运营 五页签同页），独立页保留；买家页入口 pill 按钮化；镜像重建 `shop-buyer/shop-admin:86e3144`，容器五入口 200 |
| `8eee633` | [fix][v0.2.0][T11] 支付通知兼容层支持演示页通道：识别 `channelTxnId` 别名、支持 body 内 `sign`（HMAC 订单号\|交易号\|金额），修复渠道页四个按钮全部报"参数不合法"；契约 header 签名路径与错误签名拒绝行为不变；补 contract 两用例；镜像重建 `8eee633` 容器复验 SUCCESS/paid |

> main 分支保留基线快照（`9dd5040`），整改全部落在 `dev/01-shop`；推送后对 main 发起 PR，由 CI 复核后再合并。

### 2026-11-13 batch5 整改会话更新

> - 2026-11-13 完成数据库合一 + 前后台服务拆分（8b0d468），并针对 batch5 审计 4 项新增问题完成整改：
>   I-25 版本锁定（镜像 shop-buyer/shop-admin:8b0d468 实际部署 + CI run #14）、
>   I-26 覆盖率（adminapi 0→81.4%，api 54.1%→81.3%，跨包并集 88.7%，文档改为真实复算值）、
>   I-27 管理端前端构建（npm run build + dist chunk 留证）、
>   I-28 新架构独立测试/验收/事件（51/51 复跑 + independent-test-20261113.md + events #34~#40）。

## 5. 未完成 / 待学生执行

1. 视频演示（需出镜讲解并发与退款失败重试）。
2. 群提交（仓库地址 + 验收记录）。

> 2026-11-12 21:00（batch3 后整改会话）更新：
> - 远程仓库已建（GitHub MCP：`song20060422-maker/shop-word-01`）并推送 main + dev/01-shop，CI 已实际运行（原第 3 条闭环）。
> - `-race` 已在 Linux 容器（golang:1.26.10）实际执行并留档 `docs/evidence/race-20261112.log`：全部 ok，DATA RACE 0（原第 4 条闭环）。
> - CI 首轮（run #1 @e067a24）质量门禁全过、冒烟因 `go run .` CI 全量编译超时失败，冒烟步骤已改为预编译 + 240s 等待。
