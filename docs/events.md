# AI 开发事件清单（events）

> 用途：记录题目 01 开发过程中每一次 AI 会话/工具动作的输入、产出与去向，便于验收核对"说了什么、做了什么、结果如何"。
> 时间均为本机时间（UTC+8）。事件与 `docs/dev_log.md` 的 T 编号一一对应。

| # | 时间 | 会话/角色 | 事件 | 产出 | T |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-11-11 | 开发 Agent（旧版） | 需求基线读取、工程骨架与业务实现 | `server/shop/**` 首版 | T1–T8 |
| 2 | 2026-11-11 | 开发 Agent | 51 项验收脚本实测（旧版） | 五套件 51/51，报告归档 | T8 |
| 3 | 2026-11-12 15:27 | 学生 | 本机启动旧版服务（`go run .`）做人工验证 | 8099 常驻进程 | — |
| 4 | 2026-11-12 15:30 | 开发 Agent（本次会话） | 读取审计结论（5.77/10，A3 崩溃）并定位导入环 | 修复方案：抽独立 `shop/db` 包 | T9 |
| 5 | 2026-11-12 15:32 | 开发 Agent | 新建 `shop/db/db.go`，改造 `initialize/boot.go`、`setup_test.go`；删除 `initialize/gorm.go` | 导入环解除，`go test ./shop/service` 可编译 | T9 |
| 6 | 2026-11-12 15:36 | 开发 Agent | 首次跑测试，暴露 3 个真实失败用例 | 失败清单（重复通知/异常退款/商品权限） | T9 |
| 7 | 2026-11-12 15:40 | 开发 Agent | 修正 3 处实现 + 1 处错误断言（清扫器期望值） | 全部用例通过 | T9 |
| 8 | 2026-11-12 15:44 | 开发 Agent | 统一 LF、`gofmt -w shop`、`.gitattributes` 锁行尾 | `gofmt -l shop` 空输出 | T9 |
| 9 | 2026-11-12 15:50 | 开发 Agent | 编写 HTTP 集成测试（router 包） | 覆盖登录/下单/支付/发货/退款/迟到/越权/演示重置 | T9 |
| 10 | 2026-11-12 15:58 | 开发 Agent | 集成测试发现越权漏洞（买家可读运营汇总、客服退款列表） | 修复 `OpsSummary`/`CSRefunds` 角色门禁 | T9 |
| 11 | 2026-11-12 16:05 | 开发 Agent | 补 config/middleware/db/initialize/utils/api 分支测试 | 覆盖率 66.8% → 87.0%，核心包全部 ≥85% | T9 |
| 12 | 2026-11-12 16:12 | 开发 Agent | 生成覆盖率 profile、gofmt/vet/build/test 原始输出并归档 | `docs/evidence/*` | T10 |
| 13 | 2026-11-12 16:17 | 开发 Agent | 重建二进制并重启本机服务（替换 15:27 旧进程） | 新服务健康检查通过 | T9 |
| 14 | 2026-11-12 16:19 | 开发 Agent | 真实 HTTP 验收（`run_acceptance.py`） | **51/51 通过，退出码 0**，日志归档 | T8/T10 |
| 15 | 2026-11-12 16:25 | 开发 Agent | 新增 CI 质量门禁、Shop 镜像、compose、冒烟脚本 | `.github/workflows/shop-ci.yml`、`deploy/shop/*` | T9/T10 |
| 16 | 2026-11-12 16:40 | 开发 Agent（本次会话） | 补齐 gva 框架库初始化种子（管理后台 admin 账号），初始化流程重写 config.yaml | `24f694f` | T10 |
| 17 | 2026-11-12 17:20 | 开发 Agent（本次会话） | batch1 审计整改：无鉴权端点加 demo-mode 门禁、买家区 RoleBuyer 声明与契约委派 handler 复核（I-01/I-02） | `abbcea2`（安全回归 `router/security_test.go`） | T9 |
| 18 | 2026-11-12 17:25 | 开发 Agent（本次会话） | batch1 整改：补 service 缺口用例与安全回归；修复验收脚本端口解析被其它配置段 addr 污染（I-22） | `d6b8bee` | T9 |
| 19 | 2026-11-12 17:30 | 开发 Agent（本次会话） | batch1 整改：覆盖率证据以标准单命令重生成（删除拼接 profile），总 87.3%、service 85.6/api 88.0 达标（I-03） | `5f4a8eb`，证据 `docs/evidence/coverage-single.out` | T10 |
| 20 | 2026-11-12 18:05 | 开发 Agent（本次会话） | 修复 Boot 覆盖 `global.GVA_DB` 导致管理后台登录查询 shop 库失败（端到端排查：zap error.log 定位） | `0caa980` | T9 |
| 21 | 2026-11-12 19:40 | 开发 Agent（本次会话） | batch2 整改 I-18：渠道重试/异常退款加事务级行锁闸门（并发只调渠道一次，回归测试 `service/gate_test.go`）；记款唯一键冲突才判重复支付（`isDuplicateEntry`）；callChannel 成功路径校验 RowsAffected | 本轮提交（见 dev_log §4） | T9 |
| 22 | 2026-11-12 19:50 | 开发 Agent（本次会话） | batch2 整改 I-23：契约兼容层 service 真实业务路径测试（ProductBySKU/OverwriteAvailable/双寻址/ForceExpire/异常查询/退款守卫），service 覆盖 85.6% → **87.5%**，无工具函数补洞 | 本轮提交 | T9 |
| 23 | 2026-11-12 19:55 | 开发 Agent（本次会话） | batch2 整改 I-24：验收链路强制 UTF-8（包装脚本 reconfigure + 子进程 PYTHONIOENCODING），GBK 控制台不再崩溃；无环境变量依赖复跑 **51/51 通过**，日志 `acceptance-20261112-fix2.log` | 本轮提交 | T10 |
| 24 | 2026-11-12 20:10 | 开发 Agent（本次会话） | 约束文件重构（I-07/I-21）：`.agents/dev-agent.md` 单文件拆为编号文件集 `00-index.md`~`09-bugfix.md`（技术栈/数据规则/工作流/配置部署/质量安全/自检/冲突/实现模式/缺陷修复），审计整改条款固化进对应文件；v3.0 全文归档 versions；本会话逐个编写并回读=实际加载，README 登记加载记录 | `.agents/00~09` + `dev-agent.md` v4.0 | T10 |
| 25 | 2026-11-12 20:15 | 开发 Agent（本次会话） | shop_word 工作区精简：需求三件套+假设 v1.0 归入 `references/`，旧交付（shop/、agent-test/、index.html）归入 `archive/`（只读），顶层提示词并入 `提示词/`；根 README 建立新旧路径映射 | `shop_word/README.md` | T10 |
| 26 | 2026-11-12 20:30 | 开发 Agent（本次会话） | 自编 Skill 实际调用（I-06/B3-02/03）：shop-trade-acceptance 首次调用 `check_evidence.py` 43/43 证据覆盖（产出 `evidence-check.md` + 40 项证据索引 `evidence-index.md`）；shop-trade-check 按 SKILL.md 直接调用逐套件 reset 复跑 **51/51**（日志 `acceptance-20261112-skill-call.log`）；skill-usage.md 登记调用 #4/#5 与直接/包装调用差异经验 | skill-usage.md、docs/evidence/* | T10 |
| 27 | 2026-11-12 21:14 | 开发 Agent（batch3 后整改会话） | 审计 I-19：router 层新增 HTTP 数值对账测试 `TestHTTPNumericReconciliation`（金额快照、可售占用/释放、收退款汇总、库存流水守恒 initial+Σdelta==available），本机真实 MySQL 复跑通过 | router/router_test.go | T9 |
| 28 | 2026-11-12 21:20 | 开发 Agent（batch3 后整改会话） | 审计 I-04：修复 `db/db_test.go` 三处硬编码 DSN（遵循 SHOP_TEST_DSN），Linux 容器（golang:1.26.10, CGO_ENABLED=1）执行 `go test -race -p 1 ./shop/...`，首轮发现 db 包环境性失败→修复→复跑全部 ok，DATA RACE 0；日志 `docs/evidence/race-20261112.log` | race-20261112.log | T9 |
| 29 | 2026-11-12 21:25 | 开发 Agent（batch3 后整改会话） | 审计 I-08：GitHub MCP 建仓 `song20060422-maker/shop-word-01`（记录 `docs/evidence/github-mcp-repo-created.json`），git 推送 main + dev/01-shop 成功 | 远程仓库 | T10 |
| 30 | 2026-11-12 21:31 | 开发 Agent（batch3 后整改会话） | 审计 I-12：推送触发 shop-ci 首次实际运行（run #1 @e067a24）；gofmt/vet/build/test/覆盖率/CI race 全部 success，冒烟步骤因 `go run .` 在 CI 全量编译超时失败 → 整改冒烟步骤为预编译二进制+240s 等待+失败打印日志 | .github/workflows/shop-ci.yml | T9 |
| 31 | 2026-11-12 21:22 | 开发 Agent（batch3 后整改会话） | 审计 I-17：dev_log §4 提交清单补齐 24f694f~e067a24 共 11 条；`docs/evidence/sessions/export_sessions.ps1` 将 events.md 26 条事件导出为结构化 JSONL（UTF-8），产物 `sessions/events-20261112.jsonl` | dev_log.md、sessions/events-20261112.jsonl | T10 |
| 32 | 2026-11-12 21:55 | 开发 Agent（batch3 后整改会话） | 本会话按需实际加载 `.agents` v4.0 编号文件集（00/03/05/09），README 登记加载记录 | .agents/README.md | T10 |
| 33 | 2026-11-12 22:15 | 开发 Agent（batch3 后整改会话） | 三角色 Agent 任务书入库：从 shop_word/提示词/ 复制 `agent-dev.md`（v3.0 全文，原件保留）与 `agent-acceptance.md` 到 `.agents/`，新增 `agent-test.md`（本会话独立测试实际下发任务书原文+执行记录）；README 增设"Agent 会话任务书（三角色）"章节登记三文件与实际使用记录（对应共同要求：Agent 指令版本与使用记录、三会话独立性） | .agents/agent-*.md、.agents/README.md | T10 |
| 34 | 2026-11-13 10:10 | 开发 Agent（batch5 后整改会话） | 审计 I-26（1/2）：新增 `shop/adminapi/adminapi_test.go`——GVA 管理端三分组接口真实 MySQL 测试（运营汇总/商品维护/订单核查、仓库发货与重复发货、客服退款审核/重试/异常处理、claims 注入鉴权），adminapi 覆盖 0/172 → **81.4%** | shop/adminapi/adminapi_test.go | T11 |
| 35 | 2026-11-13 10:25 | 开发 Agent（batch5 后整改会话） | 审计 I-26（2/2）：新增 `shop/api/contract_http_test.go`（api 包外部测试）契约全链路 HTTP 测试——买家下单/支付/退款、运营/仓库/客服分区与越权 403、渠道失败→重试成功、迟到支付转异常、验签失败/未知订单拒绝；api 覆盖 54.1% → **81.3%** | shop/api/contract_http_test.go | T11 |
| 36 | 2026-11-13 10:35 | 开发 Agent（batch5 后整改会话） | 审计 I-26 收口：覆盖率双口径复测——分包口径总 **82.8%**（`coverage-20261113.out`）、跨包并集总 **88.7%**（标准单命令 `-coverpkg=./shop/...`，`coverage-single-20261113.out`）；DELIVERY.md 与 `.agents/05` 更新为真实复算值并补充 `-p 1` 串行要求 | DELIVERY.md、.agents/05-quality-security.md、docs/evidence/coverage-*.out | T11 |
| 37 | 2026-11-13 10:31 | 开发 Agent（batch5 后整改会话） | 审计 I-27：`npm run build` 实际执行成功，`web/dist` 生成且 ops/warehouse/cs 三个商城后台页面 chunk 均在产物中（chunk 名与字节数留证） | docs/evidence/frontend-build-20261113.log | T11 |
| 38 | 2026-11-13 11:00 | 开发 Agent（batch5 后整改会话） | 审计 I-25：双服务镜像重建并实际部署——`shop-buyer:8b0d468`(18099) + `shop-admin:8b0d468`(18098) + admin-seed 一次性种子（gva 框架种子 + 商城后台授权 42 条 Casbin）；容器冒烟：双健康检查 200、admin 登录签发 token、/shop/ops/summary 与 /shop/ops/products code=0、买家端登录/下单/页面通过；新增 Dockerfile.gva / config.gva-docker.yaml / admin-seed.sh / gva-framework-seed.sql / init-02 留证 | docs/evidence/deploy-dual-20261113.md | T11 |
| 39 | 2026-11-13 11:05 | 开发 Agent（batch5 后整改会话） | 审计 I-28：新架构独立测试/验收留证——`py -3 scripts/run_acceptance.py --no-build --no-start` 复跑 **51/51 通过**（`acceptance-20261113-newarch.log`）；独立测试报告 `independent-test-20261113.md` 覆盖 8b0d468（取代仅覆盖旧架构 e067a24 的历史报告） | docs/evidence/independent-test-20261113.md | T11 |
| 40 | 2026-11-13 10:54 | 开发 Agent（batch5 后整改会话） | 审计 I-25 版本锁定：按类型拆分提交 `[test]5e4529b` + `[deploy]2ddb551` 并推送 dev/01-shop（8b0d468→2ddb551），触发 shop-ci **run #14 @2ddb551 全部步骤 success**（含双服务冒烟与覆盖率阈值 ≥85%），留证 `docs/evidence/ci-run14.md` / `ci-run14-jobs.json` | 远程仓库、GitHub Actions | T11 |
| 41 | 2026-11-13 14:45 | 开发 Agent（独立后台页会话） | 后台管理三页独立化（按用户要求"所有页面独立"）：新增 `shop/web/ops|warehouse|cs.html`（SHOP·OS 原型深色风，参照 archive/index.html 未改动原型），随买家端 :8090 托管；买家端服务新增 `/gva/*` 反向代理（admin-target 可配置）页内 GVA 登录调 adminapi，gva JWT+Casbin 不变；GVA Vue 三页保留双入口。提交 `c9ff4c2`[feat] + `46b124c`[fix 外链菜单 name] + `852e4ce`[deploy 容器 admin-target]，CI **run #19 @852e4ce success**；镜像重建 `shop-buyer/shop-admin:852e4ce`，容器冒烟：18099 四页面 200、/gva 反代登录 code=0、ops/summary、warehouse/shipable、cs/refunds 均 code=0 | server/shop/web/*、server/shop/router/router.go、deploy/shop/docker-compose.yaml、docs/evidence/ | T11 |
| 42 | 2026-11-13 16:20 | 开发 Agent（Skill 验收会话） | 调用 shop-trade-check + shop-trade-acceptance 双 Skill 做业务验收并补全缺口：①契约下单显式 `quantity=0` 被静默兜底成 1 件（违反 R1-4）→ 改为 0/负数/>2/非整数一律 1004 拒绝；②客服审核/异常处理允许空原因（违反 R5-2、处理不可追溯）→ 新增 `ErrReasonRequired(1005)`，空/纯空白拒绝且同意退款原因写入操作日志；③新增 `shop/router/pages_test.go` 覆盖四页面路由与 `/gva` 反代（前缀剥离、x-token 透传、非法 target 降级）。补契约账号别名 ops/warehouse/cs（种子）。实测：五组场景 **51/51 通过**（`docs/evidence/skill-check-20261113.log`）、证据核对 **43/43 无缺失**、跨包并集覆盖率 **88.4%**，报告 `docs/evidence/acceptance-report-skill-20261113.md` | server/shop/api/compat.go、service/refund.go、service/enter.go、migrations/002_seed.sql、router/pages_test.go、docs/evidence/ | T11 |
| 43 | 2026-11-13 16:05 | 开发 Agent（收尾会话） | 前端交互与新约束对齐 + 容器重建：①独立客服页 `cs.html` 修复取消被默认原因兜底的缺陷（原 `prompt()||'同意退款'` 会让"取消"变成真实审核），改为取消即中止、空白原因前端拦截；②GVA 客服页新增 `askReason`（非空校验 + 取消捕获），异常收款处理同样必填；③镜像重建 `shop-buyer/shop-admin:7309737` 并 `up -d`，容器冒烟：18099 四页面 200 且 `/cs` 含新提示、`quantity=0` → 409、空原因审核 → `code=7 客服处理必须填写原因`、带原因审核 approved+success、`ops/summary` 收款 10000/退款 10000 对账一致；UTF-8 请求体复核中文原因正常落库（此前的问号是 PowerShell 编码问题） | server/shop/web/cs.html、web/src/view/shop/cs.vue、deploy/shop/docker-compose.yaml | T11 |
| 44 | 2026-11-13 16:55 | 开发 Agent（页面入口会话） | 恢复集成版全功能页：用户反馈"打开 /shop 只有买家页签、应能看到全部页面"，而内置 `shop/web/index.html`（买家/模拟支付渠道/仓库/客服/运营 五页签同页）在拆页后一直未挂载 → 新增 `/` 与 `/index.html` 路由输出该页，独立页 `/shop` `/ops` `/warehouse` `/cs` 全部保留；`buyer.html` 顶部入口改 pill 按钮并加"全部功能（集成版）"；`pages_test.go` 补 `/` `/index.html` 断言。镜像重建 `shop-buyer/shop-admin:86e3144`，容器冒烟五个入口均 200 且根路径含五页签，容器内 `/api/products`、`/api/ops/summary|orders|ledger` 均 200。GVA 管理端 Vue 三页未改动 | server/shop/router/router.go、server/shop/web/*、server/shop/router/pages_test.go | T11 |
| 45 | 2026-11-13 17:05 | 开发 Agent（支付通知缺陷会话） | 修复买家端渠道页四个通知按钮（发送成功/重放/验签失败/金额不符）全部报"参数不合法"：契约层 `PayNotify` 只识别 `channel_txn_no|channelTxnNo` 与 `X-Channel-Signature` 头，而内置演示页发 `channelTxnId` + 请求体内 `sign`（HMAC 订单号|交易号|金额），事务号取空 → `1000 参数不合法`，演示页无法完成支付。改为双形态验签：①header 签名（HMAC 原始请求体，契约语义不变）②body sign（原生语义）；补字段别名 `channelTxnId/channel_txn_id`；错误签名仍走失败分支并留痕。实测：页面同款 `code=0 status=SUCCESS`、订单转 `paid`、错签 400；契约通道回归 `code=0`；`contract_http_test` 增加两用例。镜像重建 `shop-buyer/shop-admin:8eee633` 并容器复验通过 | server/shop/api/compat.go、server/shop/api/contract_http_test.go | T11 |
| 46 | 2026-11-14 09:10 | 开发 Agent（后台改版会话） | ①后台三页视觉对齐买家端：`ops/warehouse/cs.html` 由深色 SHOP·OS 主题改为浅色（header #111827、页面底 #f4f6fb、白卡片、表头 #f8fafc、状态 pill 实心），页签选中态改为蓝色 2px 下划线 + 加粗、未选中仅灰字、hover 只轻微变深，点击前后差异小；②登录框标注本页账号（运营 ops1 / 仓库 wh1 / 客服 cs1 或 admin），`doLogin` 校验 `authorityId`，角色不符直接拒绝并说明，缓存 token 也校验 `_aid`；③`gva-admin-shop.sql` 新增角色 9001 运营 / 9002 仓库 / 9003 客服与同名 GVA 账号（密码 123456，与 admin 同 hash）、菜单只授各自页面、Casbin 最小权限（ops→/shop/ops/**、wh→/shop/warehouse/**、cs→/shop/cs/**）。实测本地 8099 与容器 18099：ops1 登录 `code=0 aid=9001`，`/shop/ops/summary`→code=0、`/shop/cs/refunds`→code=7 被拒；四页均白底。镜像重建 `shop-buyer/shop-admin:a47947a` | server/shop/web/ops.html、warehouse.html、cs.html、deploy/shop/gva-admin-shop.sql | T11 |

## 待补充的学生事件

| 事件 | 要求 |
| --- | --- |
| 会话导出（MD/JSONL） | 在 IDE 中导出本会话，放入 `docs/evidence/sessions/`（文件名 `YYYYMMDD-会话主题.md`） |
| 视频演示 | 覆盖 S-1~S-5，重点讲解并发与退款失败重试 |
| 群提交 | 仓库地址 + 验收记录截图 |
