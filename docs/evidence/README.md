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
| D5 | 覆盖率 profile | `docs/evidence/coverage-single.out`（标准单命令生成，无拼接） | 总 **87.3%**；service 85.6%、api 88.0%（块级并集口径） |
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

## 3. D5 覆盖率（真实数据，标准单命令生成）

> 2026-11-12 审计整改（I-03）：旧 evidence 的 coverage-all.out 由多份分包 profile 拼接（同块重复出现），
> 已删除并以**单条标准命令**重新生成 `coverage-single.out`（9150 行拼接文件不再存在）。
> 分包数字为对单次 profile 按块并集聚合的结果，可复算。

```
total:      87.3%
service     85.6%   api         88.0%   router     97.8%
middleware 100.0%   config      95.9%   initialize 84.6%
model      100.0%   utils       89.2%   db         69.7%
model/response 100.0%   web 100.0%
```

说明：核心业务包 **service 85.6%、api 88.0%，均 ≥85%**（A3-03 达标）。`db`/`initialize` 为装配层
（连接/建库/迁移），未覆盖分支主要是"迁移文件读取失败"等运行期防御代码。

复现命令（单命令，count=1 禁缓存）：

```powershell
cd server
go test ./shop/... -coverprofile=coverage-single.out -coverpkg=./shop/... -count=1
go tool cover -func coverage-single.out | Select-Object -Last 1
```

## 3.1 D5-补充 安全整改回归（I-01/I-02，2026-11-12 二轮）

| 整改项 | 内容 | 验证 |
| --- | --- | --- |
| 测试钩子门禁 | `/api/test/expire-orders`、`/api/pay/channel-mode`、`/api/jobs/close-expired` 加 `shop.demo-mode` 门禁：演示环境可用（契约脚本语义不变），生产配置一律 403 | `TestDemoModeGatesTestHooks` |
| 账号列表收敛 | `GET /api/users` 仅演示模式可用（演示页选号登录依赖）；生产 403 防枚举 | 同上 |
| 买家区角色门禁 | 路由层 `RequireRole(RoleBuyer)` + 契约委派 handler（CreateOrder/CancelOrder/RefundApply）内复核 buyer 角色 | `TestBuyerRoleGate` |

验收脚本（检查者 Skill，未改动）在 demo-mode=true 下复跑 **51/51 通过**，日志 `acceptance-20261112-fix.log`。

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
