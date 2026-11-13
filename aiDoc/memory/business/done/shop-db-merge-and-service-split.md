# 商城数据库合一 + 前后台服务拆分

## 基本信息

- 提出日期：2026-11-13
- 当前状态：`done`
- 需求类型：架构调整（数据库合并 + 服务拆分）
- 优先级：高
- 需求文件：`aiDoc/memory/business/done/shop-db-merge-and-service-split.md`

## 用户原始意图摘要

把 MySQL 里散落的多个 shop 相关数据库（shop、shop_test*、shop_smoke）合并为一个：
所有表都能在 gva 库中看到；所有代码保留在 gin-vue-admin 内；
非后台管理的页面（买家端）另写一个独立 Gin 框架服务。

## 影响范围

- 后端：`server/shop`（测试库统一）、`server/cmd/shop`（新增独立服务入口）、
  `server/shop/adminapi`（新增 GVA 后台接口）、`server/shop/router/admin.go`、
  `server/initialize/router_biz.go`（不再挂买家端公开路由，改挂后台接口）
- 前端：`web/src/api/shop.js`、`web/src/view/shop/{ops,warehouse,cs}.vue`（新增）
- 文档：`server/shop/docs/DELIVERY.md`、`deploy/shop/README.md`
- 配置：`config.yaml`、`config.shop.yaml`（新增）、`config.shop-docker.yaml`、
  `config-8097/8098.yaml`、`.github/workflows/shop-ci.yml`、`Dockerfile.shop`

## 涉及对象

- 模块：shop（买家端独立服务）+ adminapi（GVA 管理端后台）
- 接口：买家端 :8090 全部契约路径（验收用）；管理端 `/shop/ops/**`、`/shop/warehouse/**`、`/shop/cs/**`
- 页面：买家端 `shop/web/buyer.html`（买家 + 模拟渠道）；管理端 Vue 三页
- 配置：业务表统一 gva 库；测试统一 `shop_test` 单库（`go test -p 1` 串行）

## 已确认约束

- 测试会 DELETE 清表，绝不能指向 gva 运行库，只能用 shop_test
- 各测试包共用 shop_test，必须 `-p 1` 串行执行
- 管理端接口走 gva JWT + Casbin；casbin 策略入 gva 库后需重启服务端生效
- 检查者脚本（shop-trade-check）不修改，契约路径保留在买家端独立服务上

## 当前进展

- 已完成：shop 11 张表（含数据）迁入 gva；删除 shop/shop_smoke/shop_test_* 共 9 个库；
  备份 `deploy/shop/backup/shop-backup-20261113.sql`
- 已完成：测试统一 shop_test（含 db_test 临时库自动 DROP、三处 reset 先清 sessions）、
  清理 shop_test 残留触发器；`go test -p 1 ./shop/...` 全绿
- 已完成：买家端独立服务 `server/cmd/shop`（:8090，页面 buyer.html），
  GVA 后台接口 adminapi + Vue 三页 + 菜单/接口/Casbin 种子 SQL（幂等）
- 已完成：CI、Dockerfile.shop、config.shop-docker.yaml、run_acceptance.py、ci-smoke.sh 同步更新
- 验证：端到端（买家下单 → 管理端核查/渠道开关）通过；51/51 业务验收通过；
  gofmt/vet/build/test 通过；前端三页 vite 编译通过

## 后续待办

- （可选）GVA 管理端容器化部署（当前 compose 只含买家端）
- （可选）生产环境把 captcha.open-captcha 调回 0（本次为脚本联调改为 1000）
- （可选）录制演示视频、群提交（历史遗留未完成项）

## 更新规则

- 同一需求始终维护在同一个文件中
- 新信息优先补充到对应段落，不要另起一份重复记录
- 只有需求状态变化时，才在 `active/` 与 `done/` 之间移动文件
