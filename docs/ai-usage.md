# AI 使用说明（D10）

> 对应约束 v3 交付项 D10：说明 AI 在本题中的参与范围、边界与不可替代的人工环节。

## 1. 参与范围

| 环节 | AI 参与 | 人工确认 |
| --- | --- | --- |
| 需求理解 | 读取题面、规则提取、状态图，生成工作项与 BR/AC 映射 | 学生确认范围与假设 |
| 编码 | 全部业务代码位于 `server/shop/`（model/service/api/router/middleware/db/migrations/web） | 学生审查与联调 |
| 测试 | 单元测试、HTTP 集成测试、并发用例、覆盖率补齐 | 学生核对断言是否符合题意 |
| 验收 | 运行自编业务 Skill（51 项）并归档日志 | 学生复跑与签字 |
| 交付证据 | 生成 D1–D10 文档与原始输出归档 | 学生补会话导出、视频、群提交 |

## 2. 边界与红线执行情况

| 红线 | 本轮执行 |
| --- | --- |
| 不编造接口/测试/覆盖率/部署记录 | 所有数据来自本机真实执行，原始输出在 `docs/evidence/` |
| 不删失败测试、不弱化断言 | 仅修正 1 处与业务事实不符的断言（D-07），其余失败均改实现 |
| 不使用未登记的任务编号 | 全部 T 编号登记在 `docs/dev_log.md`，并映射题目 01 |
| 不直推 main、不 force-push | 本轮在 `dev/01-shop` 分支提交，main 保留基线快照 |
| 密钥不入库 | 测试密钥为本地占位值，真实密钥通过配置/环境变量注入 |
| 不扩大范围 | 未实现购物车、多商户、真实物流、部分退款 |

## 3. 不可由 AI 替代的事项

1. 视频演示录制与讲解。
2. 学习群提交与答辩。
3. 远程仓库账号授权、PR 审核与合并。
4. 生产部署（本轮只交付可复现的 compose/镜像，不声明"已上线"）。

## 4. 复现指引

```powershell
# 1. 依赖与编译
cd server; go build ./...

# 2. 质量门禁
gofmt -l shop; go vet ./shop/...; go test ./shop/... -count=1

# 3. 覆盖率
go test ./shop/... -coverprofile=coverage.out -coverpkg=./shop/...
go tool cover -func=coverage.out | Select-Object -Last 1

# 4. 启动与验收
go run .            # 另开窗口
cd shop; py -3 scripts/run_acceptance.py --no-build --no-start
```
