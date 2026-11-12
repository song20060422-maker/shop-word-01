# 07 · 冲突与例外规则

## 需求矛盾

- 需求材料是只读输入；发现矛盾**停下来提请学生裁决**，记入 `docs/assumptions.md`，不得自行改需求。
- 未确认的假设不落地。

## 约束冲突

- `AGENTS.md` > 本目录编号文件 > `versions/` 历史版本。
- `AGENTS.md` 旧提交格式条款与本目录不一致时，执行本目录（`[类型][版本][T#]`），并在假设表标注待修订。

## 既有文件规则

| 对象 | 规则 |
|---|---|
| `.agents/versions/` | 只读；修改约束走"新增 vN + 更新 README 索引"，不覆盖旧版 |
| `shop_word/archive/`（旧交付 shop/、agent-test/） | 只读归档，禁止作为当前实现参考或改动对象 |
| 检查者脚本（`shop-trade-check/scripts/acceptance_check.py` 等） | **永不修改**；包装脚本只能改环境（如 PYTHONIOENCODING） |
| 检查者审计报告（audit-20261112-*） | 只读证据 |
| GVA system 模块 | 不改 |
| `C:\Users\Lenovo\Desktop\shop_word` 同名目录 | 旧副本，勿改（一切以本项目为准） |

## 编码与环境例外

- Windows GBK 控制台：Python 脚本入口强制 `sys.stdout.reconfigure(encoding="utf-8")`；子进程注入 `PYTHONIOENCODING=utf-8`。不得要求调用方手动设置环境变量（审计 I-24 整改基线）。
