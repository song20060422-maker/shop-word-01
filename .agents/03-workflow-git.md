# 03 · 工作流与 Git 约定

## 开工顺序

1. 读 `00-index.md` + 本次任务涉及的编号文件 + 需求基线材料。
2. 列出本次要实现的 BR 编号与对应 AC。
3. 提交工作项清单：`T 编号 | 对应 BR | 影响文件 | 完成判据（引用 AC）| 依赖`，确认后动工。
4. 每个工作项：实现 → 自测（按 AC 判定）→ 留证据 → 下一项。禁止一次性写完再补过程。

## 分支与提交

- 分支 `dev/<题号>`（当前 `dev/01-shop`）；禁止直推 `main`；PR 合并；禁止 force-push、跳过钩子。
- 提交格式：`[类型] [版本号] [T编号] 中文描述`；类型仅 `feat/fix/hotfix/opt/docs/test`；一次提交一种类型。
- T 编号取自 `docs/dev_log.md`，提交前核对存在，不得编造。
- 不得把功能变更伪装成 `docs`/`test`/`opt`；混合变更拆分提交。

## 提交前门禁（缺一不可）

```powershell
gofmt -l shop                 # 必须空输出
go vet ./shop/...
go build ./...
go test ./shop/... -count=1
# 涉及并发路径追加
go test -race ./shop/service/
```

## 证据随交

每工作项交付：变更文件清单（挂 BR 编号）、自测命令+真实输出+退出码、覆盖率原始 profile（单命令生成）、缺陷修正记录（写入 `docs/review_records.md`）。未真实执行的命令不得写"已验证"。
