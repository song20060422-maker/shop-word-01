# 独立测试与验收报告（新架构 8b0d468，2026-11-13）

对应审计 I-28（新架构无独立测试/验收/事件）：本报告覆盖 2026-11-13 重构提交
`8b0d468`（数据库合一 + 前后台服务拆分）之后的独立测试与验收结论，取代仅覆盖旧架构
`e067a24` 的历史报告。

## 1. 质量门禁（本机实测）

| 项 | 命令 | 结果 |
| --- | --- | --- |
| 编译 | `go build ./...` | 0 错误 |
| vet | `go vet ./shop/... ./cmd/...` | 0 告警 |
| gofmt | `gofmt -l shop cmd` | 空输出 |
| 全量测试 | `go test -p 1 ./shop/... -count=1` | 全部 ok（adminapi/api/config/db/initialize/middleware/router/service/utils） |

## 2. 新增测试（本轮补齐）

| 包 | 新增文件 | 覆盖内容 |
| --- | --- | --- |
| `shop/adminapi` | `adminapi_test.go` | GVA 管理端三组后台接口（运营/仓库/客服）成功与失败分支、claims 注入鉴权、迟到支付转异常收款的完整链路 |
| `shop/api` | `contract_http_test.go` | 契约全链路 HTTP 测试（外部测试包）：买家下单/支付/退款、运营/仓库/客服分区与越权门禁、渠道失败重试、迟到支付异常、验签失败/未知订单拒绝 |

## 3. 覆盖率（两口径，均以 -p 1 串行实测）

| 口径 | 总计 | 明细 |
| --- | --- | --- |
| 分包口径（各包仅自测，`docs/evidence/coverage-20261113.out`） | **82.8%** | adminapi 81.4 / api 81.3 / router 75.2 / service 85.5 / middleware 100 / config 95.9 / utils 89.2 / initialize 84.6 / db 81.6 |
| 跨包并集（标准单命令 `-coverpkg=./shop/...`，`docs/evidence/coverage-single-20261113.out`） | **88.7%** | CI 阈值 ≥85% 以此口径判定 |

对比审计独立复算（66.0%，adminapi 0/172）：adminapi 从 0 提升至 81.4%，api 从 54.1% 提升至 81.3%。

## 4. 业务验收（51 项）

- 命令：`py -3 shop/scripts/run_acceptance.py --no-build --no-start`（针对独立买家端 :8090）
- 结果：**51/51 通过**，日志 `docs/evidence/acceptance-20261113-newarch.log`
- 五套件：normal / stock / refund / late / races（并发 20）全部通过

## 5. 管理端前端构建（I-27）

- `npm run build` 成功（vite 构建通过），`web/dist` 生成：
  `ops.*.js`、`warehouse.*.js`、`cs.*.js` 三个商城后台页面 chunk 均在产物中
- 留证：`docs/evidence/frontend-build-20261113.log`

## 6. 双服务容器部署（I-25）

- 镜像 `shop-buyer:8b0d468` + `shop-admin:8b0d468` 重建并部署，冒烟全通过
- 留证：`docs/evidence/deploy-dual-20261113.md`

## 7. 遗留说明

- CI 触发：本次推送后由 shop-ci 实际运行（gofmt/vet/build/test/覆盖率/冒烟），运行编号见 `docs/evidence/evidence-index.md` 增补条目。
- 仍未整改项（非本轮范围）：I-10 meegle 登记、I-15 视频演示、I-20 群提交。
