# shop-ci run #14 留证（新架构双服务）

- 触发：push `2ddb551`（[deploy][v0.2.0][T11] 双服务镜像重建与部署），分支 dev/01-shop
- 运行编号：#14，run id 43355394482
- URL：https://github.com/song20060422-maker/shop-word-01/actions/runs/43355394482
- 结论：**success**（全部步骤通过）

## 步骤明细（实测）

| 步骤 | 结论 |
| --- | --- |
| 检出代码 / 安装 Go / 依赖下载 | success |
| gofmt 格式检查（自研代码） | success |
| go vet（自研代码） | success |
| 编译全仓 | success |
| 单元测试 + 集成测试 + 覆盖率 | success |
| 覆盖率阈值（≥85%） | success（跨包并集 88.7% > 85%） |
| 并发路径 race 检测 | success |
| 冒烟（买家端独立服务 + GVA 管理端） | success（8090 买家端 + 8099 管理端同时启动，契约冒烟通过） |
| 上传测试产物 | success |

原始 jobs JSON：`docs/evidence/ci-run14-jobs.json`。

说明：本次为「数据库合一 + 前后台服务拆分」后首次 CI 实际运行（此前 12 次运行与
`release-832cda4` 镜像均为旧架构），CI 冒烟步骤已同步改为双服务启动 +
`grep "shop 后台管理路由注册完成"`。
