# 交付证据索引（D1–D10）

> 原则：证据必须是原始输出，不做美化；未执行的项如实标注"未执行/待补充"，不得写"已完成"。
> 采集时间：2026-11-12（本机 Windows，MySQL 127.0.0.1:3307）。

## 1. 索引

| 编号 | 交付项 | 位置 | 状态 |
| --- | --- | --- | --- |
| D1 | 分支与提交记录 | `git log --oneline --graph dev/01-shop`、`docs/dev_log.md` §3 | 本地分支已建，提交留痕见下节 |
| D2 | 业务验收证据 | `docs/evidence/acceptance-2026-11-12.log` | 51/51 通过（退出码 0） |
| D3 | AI 事件清单 | `docs/events.md` | 已完成 |
| D4 | 自测命令与退出码 | `docs/evidence/checks-2026-11-12.log` | gofmt/vet/build/test 全通过 |
| D5 | 覆盖率 profile | `docs/evidence/coverage-all.out`、`coverage-func.txt` | 总 87.0%，核心包 ≥85% |
| D6 | race 输出 | 本文件 §4 | **本地未执行**（无 cgo），CI 执行 |
| D7 | 假设清单 | `docs/assumptions.md` | 已完成（A-01~A-19） |
| D8 | 修复与审核记录 | `docs/review_records.md` | 7 条闭环 |
| D9 | 会话导出（MD/JSONL） | `docs/prompt_export.md` + 待导出目录 `docs/evidence/sessions/` | **待学生导出** |
| D10 | AI 使用说明 | `docs/ai-usage.md` | 已完成 |

## 2. D4 原始输出摘要

见 `checks-2026-11-12.log`，关键结果：

```
gofmt -l shop                      -> <empty>            exit=0
go vet ./shop/...                  -> <no output>         exit=0
go build ./...                     -> <no output>         exit=0
go test ./shop/... -count=1        -> 全部 ok             exit=0
```

## 3. D5 覆盖率（真实数据）

```
total: 87.0%
service      90.5%   api        88.6%   router     98.4%
middleware  100.0%   config     92.4%   initialize 85.7%
model       100.0%   web       100.0%   utils      92.6%   db 72.8%
```

说明：`db` 为装配层（连接/建库），剩余未覆盖分支是"迁移文件读取失败后运行期不可能触发"的防御代码；核心业务包均 ≥85%。

复现命令：

```powershell
cd server
go test ./shop/... -coverprofile=coverage.out -coverpkg=./shop/...
go tool cover -func=coverage.out | Select-Object -Last 1
```

## 4. D6 race 说明（未执行，原因与替代）

- 本地执行：`go test -race ./shop/service -run Concurrent` → 失败，原因是 **本机 Windows 未安装 C 工具链（cgo 不可用）**，不是用例失败。
- 替代方案：CI（`.github/workflows/shop-ci.yml`）在 `ubuntu-latest` 上执行
  `go test -race ./shop/service ./shop/router -run 'Concurrent|Race|Notify|Refund|Ship|LatePayment'`。
- 推送远程后，CI 日志中的 race 结果应另存为 `docs/evidence/race-ci-<日期>.log`。

## 5. D1 提交记录（本地）

查看命令：

```bash
git log --oneline --graph --decorate -n 20
```

约定：`[类型] [版本号] [T编号] 中文描述`，类型 ∈ feat/fix/test/docs/chore/refactor。
本轮提交清单见 `docs/dev_log.md` §4（提交后由 `git log` 校验）。
