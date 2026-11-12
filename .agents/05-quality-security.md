# 05 · 质量与安全门禁

## 覆盖率

- 自编代码整体语句覆盖 ≥85%；核心包各 ≥85%：`service`、`api`、`middleware`、`config`、`initialize`、`router`。
- profile 必须以**标准单命令**生成（禁止拼接多份分包 profile）：

```powershell
go test ./shop/... -coverprofile=coverage-single.out -coverpkg=./shop/... -count=1
```

- 覆盖不足只能补**真实业务路径**用例；工具函数凑数式补洞被视为低价值（审计 I-23 教训）。
- 当前基线：总 88.0%，service 87.5%，api 88.0%。

## 测试要求

- 单元 + HTTP 集成并存；必须覆盖：正常、边界、非法状态、重复操作、权限、并发（20 并发、退款×发货、重复通知、并发重试）。
- 并发路径必须 `-race`（Linux/CI 可执行时留结果；本地无 cgo 时如实标注未执行）。
- router 层测试不得只断言状态码——关键业务数值（库存、金额）要有对账断言（审计 I-19）。

## 安全约束（审计 I-01/I-02 整改基线）

| 端点 | 约束 |
|---|---|
| `/api/test/expire-orders`、`/api/pay/channel-mode`、`/api/jobs/close-expired`、`/api/users` | `demo-mode` 门禁，生产 403 |
| 买家区 6 路由 | 路由层 `RequireRole(RoleBuyer)` **且**契约委派 handler 内复核（AuthGuard 委派先于组门禁，仅路由层拦不住） |
| 全部写操作 | 最小角色权限；越权返回业务码，不泄露细节 |

## 红线

- 不编造执行结果、测试报告、覆盖率、部署记录。
- 不删失败测试、不弱化断言、不跳过关键路径。
- 密钥/令牌/真实个人信息不进代码、日志、导出。
- 不扩大范围：购物车、多商户、真实物流、真实第三方支付。
- 模拟渠道与框架能力不得包装成自研成果。
