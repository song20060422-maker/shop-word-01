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

## 待补充的学生事件

| 事件 | 要求 |
| --- | --- |
| 会话导出（MD/JSONL） | 在 IDE 中导出本会话，放入 `docs/evidence/sessions/`（文件名 `YYYYMMDD-会话主题.md`） |
| 视频演示 | 覆盖 S-1~S-5，重点讲解并发与退款失败重试 |
| 群提交 | 仓库地址 + 验收记录截图 |
