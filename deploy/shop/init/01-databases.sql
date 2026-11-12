-- 部署初始化：服务端启动依赖 gva（gva 框架库）与 shop（业务库）两个数据库。
-- 由 mysql 官方镜像 /docker-entrypoint-initdb.d 在数据卷首次初始化时执行。
CREATE DATABASE IF NOT EXISTS gva DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS shop DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
