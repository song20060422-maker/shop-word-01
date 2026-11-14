# AGENTS.md

> 本文件是 **AI 会话（Agent）协作的唯一入口**，负责把 `.agents/` 约束文件集真正"接上电"：
> 规定按什么顺序读取、每个文件在什么场景必须读、哪些是硬性红线、加载后如何留痕。
>
> 与根目录 `AGENT.MD` 的分工：`AGENT.MD` 管理 `aiDoc/` 文档层（架构说明、示例、记忆）；
> 本文件管理 `.agents/` 约束层（业务规则矩阵、流程、质量门禁、审查清单、缺陷流程）。
> 两者内容冲突时的裁决见第 6 节。

## 1. 会话启动：必读顺序

任何新会话，**第一步**按下列顺序加载，不得跳步：

1. `AGENTS.md`（本文件）
2. `.agents/00-index.md`（约束文件集索引与阅读顺序）
3. `.agents/01-stack-structure.md`（技术栈、目录分层、允许改动边界）
4. 按第 2 节"任务→文件"映射，选读本次任务相关编号文件（**不是每次全读**）
5. 若本会话承担开发/测试/验收角色之一，整体加载对应任务书：
   - 开发：`.agents/agent-dev.md`（v3.0 全文任务书）
   - 独立测试：`.agents/agent-test.md`
   - 独立验收：`.agents/agent-acceptance.md`

上下文入口（只读，不作为约束）：`shop_word/references/` 四份需求材料、`docs/assumptions.md`、`shop/docs/DELIVERY.md`。

## 2. 任务 → 必读文件映射

| 本次任务 | 必须读取 | 说明 |
| --- | --- | --- |
| 每次会话开始 | `00-index.md` | 索引与阅读顺序 |
| 首次接触工程 / 改目录结构 | `01-stack-structure.md` | 分层为 Router → API → Service → Model；明确可改/不可改边界 |
| 动任何订单状态、库存、支付、退款、发货字段 | `02-data-rules.md` | BR-01~12 业务矩阵、E-01~12 异常矩阵、INV-1~4 不变量，改状态前逐条对照 |
| 写 / 改 `shop/service` 下的业务流程 | `08-flow-codegen.md` | 条件 UPDATE、幂等闸门、错误分类的实现范式 |
| 每次提交前 | `03-workflow-git.md` | 分支策略、T 编号来源、提交格式 `[类型][版本][T编号] 描述` |
| 改配置 / Dockerfile / compose / 部署 | `04-deploy-config.md` | `config.yaml` shop 段、演示模式、双服务部署约定 |
| 每次自测 / 声称覆盖率 | `05-quality-security.md` | 覆盖率门禁、`-p 1` 串行、race、角色权限与安全红线 |
| 交接 / 交付前 | `06-review-checklist.md` | 自检清单与交接格式 |
| 需求、文档、历史版本互相矛盾 | `07-conflict-rules.md` | 矛盾裁决顺序：题面 > 可审计事实 > 文档；旧交付版本处理规则 |
| 修缺陷 | `09-bugfix.md` | 修复流程、根因说明、留痕与回归要求 |

约束文件本身的历史与版本演进见 `.agents/README.md`（版本索引）与 `.agents/versions/`（v1→v3 只读归档）。

## 3. 硬性红线（摘自 `.agents`，任何会话不得违反）

- **数据库**：业务表在 `gva` 单库，测试只用 `shop_test` 库；测试包必须 `-p 1` 串行执行（同库清表会互踩，含覆盖率命令）。
- **测试真实性**：只补真实业务路径用例，禁止为凑覆盖率写只调工具的假用例（`.agents/05` 审计 I-23 教训）。
- **覆盖率口径**：对外宣称必须用标准单命令（`-coverpkg=./shop/...`）实测值，分包与跨包并集两个口径都要注明，禁止拼接或估算。
- **状态机**：状态流转只能用条件 UPDATE + 幂等闸门，禁止"先查后写"的竞态实现（`.agents/02` / `08`）。
- **权限**：买家、运营、仓库、客服用例必须各自验证越权被拒（403/401），不得只测 happy path。
- **提交**：一次提交只做一类事，格式 `[类型][v0.2.0][T编号] 描述`，T 编号必须能在 `docs/dev_log.md` 查到；禁止 `--amend` 覆盖已推送历史。
- **证据**：声称"通过/已部署/已构建"必须留下可复核产物（日志、profile、CI 运行号），禁止口头结论。
- **代码读取**：不读 `node_modules/`；第三方行为查调用点、锁文件或官方文档。

## 4. 三角色独立性

- 开发、独立测试、独立验收可以同工具同模型，但**必须是相互独立的会话**。
- 测试与验收必须读取**原始题面**与被测**具体提交版本**代码，不得复用开发会话的结论。
- 三者产物落点：测试报告 `.agents/../agent-test/`，验收报告见交付材料；实际使用记录登记在 `.agents/README.md` 的"Agent 会话任务书（三角色）"表与"加载记录"表。

## 5. 加载留痕（强制）

- 会话中**真实读取**了哪些约束文件，须在 `.agents/README.md` 的"加载记录"新增一行：时间、会话、加载版本/编号、对应任务、结果。
- 只记录真实发生的加载；未发生的会话不得补写。
- 修改约束本身：先新增 `.agents/versions/<agent>.vN.md`，再更新 `.agents/README.md` 与生效文件，**禁止就地覆盖历史版本**；写明改了什么、为什么改、对应 T 编号。
- 事件与工作项登记：`docs/events.md`（事件流水）、`docs/dev_log.md`（T 编号与提交清单）、`docs/review_records.md`（缺陷与整改）。

## 6. 冲突裁决

1. 需求冲突：原始题面 > 可审计事实（代码/配置/日志）> 项目文档（`.agents/07-conflict-rules.md`）。
2. 文档层冲突：`AGENTS.md`（本文件，.agents 约束层）与 `AGENT.MD`（aiDoc 文档层）就同一问题给出不同要求时——**代码、测试、提交、部署相关以本文件 + `.agents/` 为准；aiDoc 的组织说明、示例写法以 `AGENT.MD` 为准**。
3. 与交付实现冲突：以仓库中可复核的代码、profile、CI 运行记录为准，文档随后同步修正（不得反过来改文档掩盖事实）。

## 7. 本仓库当前事实（2026-11-13 拆分后，供快速定位，细节以 `.agents/01` 为准）

- `server/cmd/shop`：买家端 + 模拟支付渠道独立服务（本地 `:8080`，与 cpolar 隧道对齐、支付宝回调直接走公网域名；容器 `:18099`），`config.shop.yaml` / `config.shop-docker.yaml`。
- `server/shop/`：买家端业务（`api` / `service` / `model` / `db` / `migrations` / `scripts`）与后台接口 `adminapi`（运营/仓库/客服，GVA `:8099` 容器 `:18098`，`shop/router/admin.go` 挂 `/shop/**`）。
- `web/src/view/shop/`：管理端三个后台页面 `ops.vue` / `warehouse.vue` / `cs.vue`。
- `deploy/shop/`：`docker-compose.yaml`（mysql + redis + shop-buyer + shop-admin + admin-seed）、`gva-admin-shop.sql`、`gva-framework-seed.sql`、`admin-seed.sh`。
- `docs/`：`events.md`、`dev_log.md`、`review_records.md`、`evidence/`（覆盖率 profile、CI 运行留证、验收日志）。

## 8. 文档索引

- 约束层（本文件管辖）：`.agents/00-index.md` ~ `.agents/09-bugfix.md`、`.agents/agent-*.md`、`.agents/README.md`、`.agents/versions/`
- 文档层（`AGENT.MD` 管辖）：`aiDoc/README.md`、`aiDoc/relations/`、`aiDoc/modules/`、`aiDoc/frontend-backend/`、`aiDoc/examples/`、`aiDoc/memory/`
- 交付层：`shop/docs/DELIVERY.md`、`shop/docs/DESIGN.md`、`docs/evidence/evidence-index.md`
