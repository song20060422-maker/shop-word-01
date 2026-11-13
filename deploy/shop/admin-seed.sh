#!/bin/sh
# gva 管理端首次部署种子脚本（compose 的 admin-seed 一次性服务执行）：
#   1. 等待 admin 服务启动完成（gva 框架表由启动流程自动迁移）
#   2. 若 sys_users 为空，注入 gva 框架种子（gva-framework-seed.sql：admin 账号、
#      角色/菜单/API/Casbin 基线，来源于本地已初始化库的 mysqldump）
#   3. 注入商城后台菜单/接口/Casbin 授权（gva-admin-shop.sql，幂等）
# 幂等：可重复执行；重复执行时 INSERT 可能报唯一键冲突（框架种子），不影响已就绪状态。
set -e

echo "[admin-seed] 等待 gva 框架表初始化..."
i=0
until mysql -hmysql -uroot -proot -N -e "select 1 from gva.sys_users limit 1" >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -gt 60 ]; then
    echo "[admin-seed] 等待超时（admin 服务未就绪）"
    exit 1
  fi
  sleep 5
done

cnt=$(mysql -hmysql -uroot -proot -N -e "select count(*) from gva.sys_users")
if [ "$cnt" = "0" ]; then
  echo "[admin-seed] 注入 gva 框架种子数据..."
  mysql -hmysql -uroot -proot --default-character-set=utf8mb4 gva < /seed/gva-framework-seed.sql || \
    echo "[admin-seed] 框架种子部分冲突，跳过（幂等）"
fi

mysql -hmysql -uroot -proot --default-character-set=utf8mb4 gva < /seed/gva-admin-shop.sql
echo "[admin-seed] 商城后台菜单/接口/授权就绪："
mysql -hmysql -uroot -proot -N -e "select count(*) from gva.casbin_rule where v1 like '/shop/%'"
