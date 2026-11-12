# 04 · 配置与部署

## config.yaml shop 段（关键项）

| 项 | 值 | 说明 |
|---|---|---|
| `addr` | ":8099" | 服务监听端口 |
| `demo-mode` | true | 演示模式：测试钩子与 `/api/users` 可用；**生产必须 false**（一律 403） |
| `mysql` | 3307/shop | 业务库连接 |
| `pay-notify-secret` | 演示密钥 | HMAC 支付通知验签；不入代码、不打日志 |

> 坑：配置初始化会重写 config.yaml，其它段可能出现 `addr:`——脚本解析端口必须**限定 shop 段**（见 `scripts/run_acceptance.py read_base_url`）。

## 部署约定

- CI：`.github/workflows/shop-ci.yml`（lint/build/test/coverage 门禁），未 push 前不会实际运行——文档如实标注，不得写"已验证"。
- 容器：`deploy/shop/`（Dockerfile + compose + 冒烟脚本）。
- 未真实部署前，DELIVERY/README 保持"未部署"标注。

## 本地运行

```powershell
cd server
go build -o bin\shop-server.exe .
.\bin\shop-server.exe     # 8099
# 验收（无环境变量依赖，脚本内部强制 UTF-8）
py -3 shop\scripts\run_acceptance.py --no-build --no-start
```

演示账号：`ops1 / wh1 / cs1 / buyer1~3`，口令 `shop123`；商城页 `http://127.0.0.1:8099/shop`。
