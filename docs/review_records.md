# 审核与缺陷修正记录（review_records）

> 规则：每条问题按「发现 → 根因 → 修正 → 复测证据」闭环，不允许只写"已修复"。
> 复测命令均在 `server/` 目录执行；原始输出见 `docs/evidence/checks-2026-11-12.log`。

## 1. 缺陷清单（2026-11-12 第二轮审计整改）

| # | 严重度 | 问题 | 违反条目 | 状态 |
| --- | --- | --- | --- | --- |
| D-01 | P0 | `go test ./shop/service` 编译失败：测试导入环，14 个用例无法运行 | A3 质量门禁、验收 2.1 M2 | 已修复 |
| D-02 | P0 | 24 个文件 gofmt 不合规（CRLF 行尾） | 质量门禁（gofmt/vet） | 已修复 |
| D-03 | P1 | 买家可读取运营汇总 `GET /api/ops/summary`、客服退款列表 `GET /api/cs/refunds` | E-10 越权、AC-08 | 已修复 |
| D-04 | P1 | 异常退款重试必然失败：`refunds.order_no` 唯一键冲突 | BR-09、E-08、AC-06 | 已修复 |
| D-05 | P1 | 重复支付通知返回 `err=nil`，调用方无法判定"未重复记款" | BR-05、E-03、AC-04 | 已修复 |
| D-06 | P1 | 买家可调用商品维护（缺角色门禁） | E-10、BR-12 | 已修复 |
| D-07 | P2 | 测试断言错误：清扫器期望关闭 1 笔，实际环境有 3 笔待付款单同时超时 | 测试自身缺陷（此前从未执行） | 已修正断言 |

## 2. 逐条闭环

### D-01 测试导入环

- 发现：`go test ./shop/service` 报 import cycle：`service(test) → initialize → service`。
- 根因：`initialize/boot.go` 为装配 `ServiceGroup` 反向依赖 `service`，而测试辅助（连接/迁移/种子）与 `Boot` 同包。
- 修正：
  - 新增 `server/shop/db/db.go`：`Gorm`/`Migrate`/`SeedAccounts`/`splitSQL`；
  - `initialize/boot.go` 只做装配，依赖 `shop/db`；
  - 删除 `initialize/gorm.go`；
  - 三个测试包（service/router/api）统一用 `SHOP_TEST_DSN` 或默认独立库。
- 复测：`go test ./shop/... -count=1` → 全部 `ok`，退出码 0。

### D-02 gofmt / CRLF

- 发现：`gofmt -l shop` 全量命中。
- 根因：Windows 检出自动转换 CRLF，且部分文件未格式化。
- 修正：全部 `.go` 转 LF；`gofmt -w shop`；`.gitattributes` 增加 `*.go text eol=lf`；CI 增加 gofmt 门禁。
- 复测：`gofmt -l shop` 输出为空；`go vet ./shop/...` 退出码 0。

### D-03 越权读取运营/客服数据（集成测试发现）

- 发现：`TestOpsMaintainAndReport` 断言买家访问 `/api/ops/summary` 应 403，实测 200。
- 根因：契约兼容层通过 `delegate()` 在路由链最前面接管请求（`c.Abort()`），绕过了 `ops`/`cs` 路由组上的 `RequireRole`。
- 修正：`CompatApi.OpsSummary`、`CompatApi.CSRefunds` 内部复核 `middleware.ClaimsOf(c).Role`，不符返回 2002。
- 复测：同一用例断言 403 通过；另加"买家查退款列表 403"用例。

### D-04 异常退款重试失败

- 发现：`TestLatePaymentAbnormalRefund` 第二次处理报"该订单已退款或已在退款流程中"。
- 根因：`HandleAbnormal` 每次都 `Create` 新退款单，而 `refunds.order_no` 唯一键要求一单一退款单；首次渠道失败已落库。
- 修正：事务内先查该订单退款单，失败/待处理状态则复用并重置为 `pending` 后重试，仅成功态拒绝。
- 复测：失败→重试成功后 `channel_status=success`、异常记录 `handled`、库存不变。

### D-05 重复通知错误语义

- 发现：`TestNotifyRules` 期望 `ErrDuplicatedPay`，实测 `err=nil`。
- 根因：重复分支在事务内写留痕后返回 `nil`，外层未映射哨兵错误。
- 修正：事务提交后若 `result.Result == duplicated` 返回 `ErrDuplicatedPay`（留痕不丢）。
- 复测：重复通知返回 3003/409，`payments` 计数仍为 1。

### D-06 买家维护商品

- 发现：`TestCatalogAndAuthAndDemoReset` 期望 `ErrForbidden`，实测 `nil`。
- 根因：`CatalogService.Update` 只校验价格，未校验角色。
- 修正：入口校验 `actorRole == ops`，否则 2002。
- 复测：买家 403、运营成功、未知商品 404。

### D-07 断言错误（测试自身）

- 发现：`TestNotifyRules` 在超时清扫处期望 `n==1`，实测 3。
- 根因：此时 o2/o3/o4 三笔待付款单同时超时，实现行为正确，断言写错（该用例此前因编译失败从未执行）。
- 修正：断言改为 3 并注明原因；未删除任何用例、未跳过路径。
- 复测：用例通过。

## 3. 本轮新增验证

| 验证 | 命令 | 结果 |
| --- | --- | --- |
| 全量测试 | `go test ./shop/... -count=1` | 11 个包通过（4 个无测试文件），退出码 0 |
| 覆盖率 | `go test ./shop/... -coverprofile=... -coverpkg=./shop/...` | 87.0%（service 90.5 / api 88.6 / router 98.4 / middleware 100 / config 92.4 / initialize 85.7） |
| 业务验收 | `py -3 shop/scripts/run_acceptance.py --no-build --no-start` | 51/51，退出码 0 |
| 编译 | `go build ./...` | 退出码 0 |
| race | `go test -race ...` | 本机无 cgo 工具链未执行，CI 执行（见 evidence README） |
