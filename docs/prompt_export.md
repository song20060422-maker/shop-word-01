# 提示词与会话导出（prompt_export）

> 对应交付项：D9（会话导出 MD + JSONL）、B1（提示词导出）。本文件说明"用了什么提示词、怎么导出、导出到哪"。

## 1. 会话提示词结构

| 层 | 文件 | 说明 |
| --- | --- | --- |
| 开发约束（首条提示词） | `.agents/dev-agent.md`（v3.0） | 角色、开工顺序、BR/E/S 矩阵、门禁、交付证据 D1–D10、红线 |
| 开发约束历史版本 | `.agents/versions/dev-agent.v1.md`、`v2.md`、`v3.md` | 约束演化记录（v1 初版 → v3 增加 D1–D10 与门禁） |
| 约束加载说明 | `.agents/README.md` | 何时加载、如何引用 |
| 修复会话指令 | 见下方 §3 | 本轮"按审计结论整改"的实际输入 |

## 2. 导出方式（IDE 内操作）

1. 打开本任务会话，点击会话面板右上角「导出」→ 选择 **Markdown**，保存为
   `docs/evidence/sessions/20261112-审计整改.md`。
2. 同一面板选择 **JSONL（原始事件流）**，保存为
   `docs/evidence/sessions/20261112-审计整改.jsonl`。
3. 导出文件不包含密钥/令牌；如含手机号等敏感信息需先脱敏再入库。

> 未导出前，本文件与 `docs/events.md` 已提供可核对的事件序列；导出后请把文件名补到 `docs/evidence/README.md` 的 D9 行。

## 3. 本轮实际提示词（原文）

审计输入（截图转录）：

> 第二轮检查完成。最终得分 5.77/10，不合格。好信息：业务与数据这一版是真的过硬（BR-01~12 全部正确实现；五场景实测全过 51/51；重启持久化实测通过）。
> 导致不合格的硬伤：1) 测试导入环，`setup_test.go` 引 `shop/initialize`，`boot.go` 反向引 `shop/service`，`go test ./shop/service` 编译失败，14 个测试全部无法运行；2) gofmt 全部 24 个文件不合规（CRLF）+ vet 失败；3) 无 CI/部署/视频/群提交；4) 单次快照提交直推 main、无远程/PR/dev 分支；5) 提示词导出/事件清单/skill-usage.md/审核记录全缺；6) 约束 v3 用内部 T 编号替代题目飞书编号 + meegle 核验。
> 修复建议：P0 拆导入环 → 测试能跑 → 统一 LF → 提交 profile 和 race 输出。

开发指令：

> 根据截图接着改。

## 4. 本轮提示词执行映射

| 审计要求 | 执行动作 | 证据 |
| --- | --- | --- |
| P0 拆导入环 | 新增 `shop/db` 包；`Boot` 与测试共同依赖 | `shop/db/db.go`、`docs/evidence/checks-2026-11-12.log` |
| 测试能跑 | 修复 3 个真实失败 + 1 处错误断言 | `docs/review_records.md` |
| 统一 LF | 全部转 LF + gofmt + `.gitattributes` | `gofmt -l shop` 空输出 |
| 覆盖率 | 66.8% → 87.0%，核心包 ≥85% | `docs/evidence/coverage-all.out` |
| race | 本地缺 cgo 未能执行，改由 CI | `docs/evidence/README.md` §D6 |
| CI/部署 | `shop-ci.yml`、`Dockerfile.shop`、compose、冒烟脚本 | `.github/workflows/`、`deploy/shop/` |
| 编号映射 | 工作项同时标注题目 01 与 BR/AC | `docs/dev_log.md` §1 |
