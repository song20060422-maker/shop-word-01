-- 部署初始化：业务表统一放在 gva 库（gva 框架表 + shop 业务 11 张表共用一个库）。
-- 由 mysql 官方镜像 /docker-entrypoint-initdb.d 在数据卷首次初始化时执行。
-- gva 框架表由 GVA 主服务启动时自动迁移；shop 业务表由服务启动时执行 shop/migrations/*.sql。
CREATE DATABASE IF NOT EXISTS gva DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
