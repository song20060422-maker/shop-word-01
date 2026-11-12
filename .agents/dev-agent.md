# 开发 Agent 约束（入口文件）

> v4.0（2026-11-12）：单文件拆分为编号文件集（格式参考编号式规则目录，内容为本项目实际约束）。
> 历史版本见 `versions/`（v1.0 独立项目基线 → v2.0 GVA 结合版 → v3.0 需求驱动版），**只读**。

## 使用方式

开发会话按需加载以下编号文件（先读 `00-index.md`）：

| 文件 | 主题 |
|---|---|
| `00-index.md` | 索引、阅读顺序、加载方式 |
| `01-stack-structure.md` | 技术栈、目录边界、允许改动范围 |
| `02-data-rules.md` | 状态机、BR/E 矩阵、不变量、并发防御约束 |
| `03-workflow-git.md` | 工作项、分支、提交规范、提交前门禁 |
| `04-deploy-config.md` | config.yaml shop 段、demo-mode、部署约定 |
| `05-quality-security.md` | 覆盖率门禁、race、安全基线、红线 |
| `06-review-checklist.md` | 提交前自检清单、交接格式 |
| `07-conflict-rules.md` | 需求矛盾裁决、只读对象清单、编码例外 |
| `08-flow-codegen.md` | service 实现模式（条件 UPDATE/幂等/闸门/留痕） |
| `09-bugfix.md` | 缺陷修复流程、教训库 |

## 需求基线（只读，冲突时以题面原文为准）

| 材料 | 路径 |
|---|---|
| 需求说明 | `shop_word/references/需求文档-Shop商城小型交易系统.md` |
| 规则提取 | `shop_word/references/business-rules.md` |
| 状态与流程 | `shop_word/references/证据-业务流程与状态图.md` |
| 原型说明 | `shop_word/references/页面状态说明.md` |
| 假设清单 | `docs/assumptions.md`（随工程） |

> v3.0 全文（BR/E/S/AC/INV 逐条绑定判据）已归档于 `versions/dev-agent.v3.md`，需要逐条判据时读它。
