# 00 · 索引与阅读顺序

> 本目录是 Shop 商城（题目 01）开发 Agent 的约束文件集，按编号拆分，各管一件事。
> 只参考编号格式组织内容，内容均为本项目实际约束。

## 文件清单与阅读顺序

| 编号 | 文件 | 内容 | 何时读 |
|---|---|---|---|
| 00 | `00-index.md` | 本索引 | 每次会话开始 |
| 01 | `01-stack-structure.md` | 技术栈、目录分层与允许改动边界 | 首次接触工程 / 新会话 |
| 02 | `02-data-rules.md` | 业务规则矩阵（BR）、异常矩阵（E）、不变量（INV） | 动任何状态字段前 |
| 03 | `03-workflow-git.md` | 分支、工作项（T 编号）、提交规范 | 每次提交前 |
| 04 | `04-deploy-config.md` | 配置（config.yaml shop 段）、演示模式、部署约定 | 改配置/部署前 |
| 05 | `05-quality-security.md` | 覆盖率门禁、race、角色与安全约束、红线 | 每次自测前 |
| 06 | `06-review-checklist.md` | 提交前自检清单与交接格式 | 每次交接前 |
| 07 | `07-conflict-rules.md` | 需求矛盾裁决、历史版本与旧交付规则 | 遇到冲突时 |
| 08 | `08-flow-codegen.md` | 业务流实现模式（条件 UPDATE、幂等闸门、错误分类） | 写 service 代码时 |
| 09 | `09-bugfix.md` | 缺陷修复流程与留痕要求 | 修缺陷时 |

## 加载方式

- 开发会话首条消息：读本 `00-index.md` → 按本次任务选读 01/02/08（改动涉及状态机时必读 02+08）。
- 需求基线（只读）：`shop_word/references/` 四份材料 + 工程 `docs/assumptions.md`。
- 历史版本：`versions/`（只读，v1→v3 演进见 `README.md` 版本索引）。

## 与其它约束的关系

- `AGENTS.md`（工程根）：全局约束；冲突时以 `AGENTS.md` 为准。
- `docs/dev_log.md`：T 编号唯一来源；提交编号必须能在其中查到。
- `docs/review_records.md`：缺陷与整改留痕目的地。
