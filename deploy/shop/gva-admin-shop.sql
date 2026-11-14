-- ============================================================
-- Shop 商城后台管理（GVA 管理端）菜单 / 接口 / Casbin 授权初始化
-- 幂等：重复执行不会产生重复数据。
-- 适用库：gva（业务表与框架表共用一个库）。
-- 执行：mysql -h127.0.0.1 -P3307 -uroot -proot gva < deploy/shop/gva-admin-shop.sql
-- 注意：Casbin 策略在服务启动时加载，执行后需重启 GVA 服务端生效。
-- ============================================================
SET NAMES utf8mb4;

-- ---------- 1. 菜单：商城管理（目录）+ 运营核查 / 仓库发货 / 客服退款 ----------
INSERT INTO sys_base_menus (created_at, updated_at, deleted_at, menu_level, parent_id, path, name, hidden, component, sort, active_name, keep_alive, default_menu, title, icon, close_tab)
SELECT NOW(3), NOW(3), NULL, 0, 0, 'shop', 'shopAdmin', 0, 'view/routerHolder.vue', 6, '', 0, 0, '商城管理', 'operation', 0
WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE name = 'shopAdmin' AND deleted_at IS NULL);

INSERT INTO sys_base_menus (created_at, updated_at, deleted_at, menu_level, parent_id, path, name, hidden, component, sort, active_name, keep_alive, default_menu, title, icon, close_tab)
SELECT NOW(3), NOW(3), NULL, 0, (SELECT id FROM (SELECT id FROM sys_base_menus WHERE name = 'shopAdmin' AND deleted_at IS NULL) t), 'ops', 'shopOps', 0, 'view/shop/ops.vue', 1, '', 0, 0, '运营核查', 'pie-chart', 0
WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE name = 'shopOps' AND deleted_at IS NULL);

INSERT INTO sys_base_menus (created_at, updated_at, deleted_at, menu_level, parent_id, path, name, hidden, component, sort, active_name, keep_alive, default_menu, title, icon, close_tab)
SELECT NOW(3), NOW(3), NULL, 0, (SELECT id FROM (SELECT id FROM sys_base_menus WHERE name = 'shopAdmin' AND deleted_at IS NULL) t), 'warehouse', 'shopWarehouse', 0, 'view/shop/warehouse.vue', 2, '', 0, 0, '仓库发货', 'management', 0
WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE name = 'shopWarehouse' AND deleted_at IS NULL);

INSERT INTO sys_base_menus (created_at, updated_at, deleted_at, menu_level, parent_id, path, name, hidden, component, sort, active_name, keep_alive, default_menu, title, icon, close_tab)
SELECT NOW(3), NOW(3), NULL, 0, (SELECT id FROM (SELECT id FROM sys_base_menus WHERE name = 'shopAdmin' AND deleted_at IS NULL) t), 'cs', 'shopCs', 0, 'view/shop/cs.vue', 3, '', 0, 0, '客服退款', 'service', 0
WHERE NOT EXISTS (SELECT 1 FROM sys_base_menus WHERE name = 'shopCs' AND deleted_at IS NULL);

-- ---------- 2. 菜单授权（888 管理员 / 9528 演示管理员；表仅两列） ----------
INSERT INTO sys_authority_menus (sys_base_menu_id, sys_authority_authority_id)
SELECT m.id, a.aid
FROM sys_base_menus m
JOIN (SELECT 888 AS aid UNION SELECT 9528) a
WHERE m.name IN ('shopAdmin', 'shopOps', 'shopWarehouse', 'shopCs') AND m.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_authority_menus am
                  WHERE am.sys_base_menu_id = m.id AND am.sys_authority_authority_id = a.aid);

-- ---------- 2.1 历史外链菜单修正：买家端已拆分为独立 Gin 服务（:8090） ----------
-- gva 前端判断外链看的是菜单 name（normalMode.vue: index = routerInfo.name，含 http 才 window.open），
-- 只改 path 会导致 permission.js 跳过路由注册 + router.push({name:'shopDemo'}) 找不到路由 → 点击无反应。
-- 因此 name 与 path 必须同时置为 URL。
UPDATE sys_base_menus
SET name = 'http://127.0.0.1:8090/shop', path = 'http://127.0.0.1:8090/shop',
    title = '商城买家端', icon = 'shop', updated_at = NOW(3)
WHERE name IN ('shopDemo', 'http://127.0.0.1:8090/shop') AND deleted_at IS NULL;

-- ---------- 3. 接口注册（sys_apis，分组「商城管理」） ----------
INSERT INTO sys_apis (created_at, updated_at, deleted_at, path, description, api_group, method)
SELECT NOW(3), NOW(3), NULL, x.path, x.description, '商城管理', x.method
FROM (
  SELECT '/shop/ops/summary' AS path, '交易汇总' AS description, 'GET' AS method
  UNION SELECT '/shop/ops/products', '商品列表（含下架）', 'GET'
  UNION SELECT '/shop/ops/products/:id', '商品维护（上下架/调价/覆盖可售）', 'PATCH'
  UNION SELECT '/shop/ops/orders', '订单列表', 'GET'
  UNION SELECT '/shop/ops/orders/:orderNo', '订单全链路核查', 'GET'
  UNION SELECT '/shop/ops/notices', '支付通知记录', 'GET'
  UNION SELECT '/shop/ops/abnormals', '异常收款列表', 'GET'
  UNION SELECT '/shop/ops/ledger', '库存流水', 'GET'
  UNION SELECT '/shop/ops/trail', '操作留痕', 'GET'
  UNION SELECT '/shop/warehouse/shipable', '待发货列表', 'GET'
  UNION SELECT '/shop/warehouse/ship', '发货登记', 'POST'
  UNION SELECT '/shop/warehouse/shipments', '发货记录', 'GET'
  UNION SELECT '/shop/warehouse/orders/:orderNo', '订单详情（仓库）', 'GET'
  UNION SELECT '/shop/cs/refunds', '退款单列表', 'GET'
  UNION SELECT '/shop/cs/refunds/audit', '退款审核', 'POST'
  UNION SELECT '/shop/cs/refunds/retry', '退款重试', 'POST'
  UNION SELECT '/shop/cs/abnormals', '异常收款列表（客服）', 'GET'
  UNION SELECT '/shop/cs/abnormals/handle', '异常收款处理（发起退款）', 'POST'
  UNION SELECT '/shop/cs/notices', '支付通知记录（客服）', 'GET'
  UNION SELECT '/shop/cs/orders/:orderNo', '订单详情（客服）', 'GET'
  UNION SELECT '/shop/cs/channel/mode', '模拟渠道退款模式切换', 'POST'
) x
WHERE NOT EXISTS (SELECT 1 FROM sys_apis a2 WHERE a2.path = x.path AND a2.method = x.method AND a2.deleted_at IS NULL);

-- ---------- 4. Casbin 授权（888 / 9528） ----------
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', a.aid, x.path, x.method
FROM (
  SELECT '/shop/ops/summary' AS path, 'GET' AS method
  UNION SELECT '/shop/ops/products', 'GET'
  UNION SELECT '/shop/ops/products/:id', 'PATCH'
  UNION SELECT '/shop/ops/orders', 'GET'
  UNION SELECT '/shop/ops/orders/:orderNo', 'GET'
  UNION SELECT '/shop/ops/notices', 'GET'
  UNION SELECT '/shop/ops/abnormals', 'GET'
  UNION SELECT '/shop/ops/ledger', 'GET'
  UNION SELECT '/shop/ops/trail', 'GET'
  UNION SELECT '/shop/warehouse/shipable', 'GET'
  UNION SELECT '/shop/warehouse/ship', 'POST'
  UNION SELECT '/shop/warehouse/shipments', 'GET'
  UNION SELECT '/shop/warehouse/orders/:orderNo', 'GET'
  UNION SELECT '/shop/cs/refunds', 'GET'
  UNION SELECT '/shop/cs/refunds/audit', 'POST'
  UNION SELECT '/shop/cs/refunds/retry', 'POST'
  UNION SELECT '/shop/cs/abnormals', 'GET'
  UNION SELECT '/shop/cs/abnormals/handle', 'POST'
  UNION SELECT '/shop/cs/notices', 'GET'
  UNION SELECT '/shop/cs/orders/:orderNo', 'GET'
  UNION SELECT '/shop/cs/channel/mode', 'POST'
) x
JOIN (SELECT 888 AS aid UNION SELECT 9528) a
WHERE NOT EXISTS (SELECT 1 FROM casbin_rule c
                  WHERE c.ptype = 'p' AND c.v0 = a.aid AND c.v1 = x.path AND c.v2 = x.method);

-- ---------- 5. 角色账号：运营 / 仓库 / 客服（独立后台页专用，最小权限） ----------
-- 说明：ops1/wh1/cs1 在买家端 Gin 服务里是业务账号；这里在 GVA 侧建同名账号 + 独立角色，
--       使「运营核查」页只能由运营账号进入、「仓库发货」页只能由仓库账号进入，依此类推。
-- 密码统一 123456（与 admin 同 hash），仅用于演示环境；生产需改密并关闭演示入口。
INSERT INTO sys_authorities (created_at, updated_at, deleted_at, authority_id, authority_name, parent_id, default_router)
SELECT NOW(3), NOW(3), NULL, x.aid, x.aname, 0, 'shopAdmin'
FROM (
  SELECT 9001 AS aid, '运营' AS aname
  UNION SELECT 9002, '仓库'
  UNION SELECT 9003, '客服'
) x
WHERE NOT EXISTS (SELECT 1 FROM sys_authorities a WHERE a.authority_id = x.aid AND a.deleted_at IS NULL);

INSERT INTO sys_users (created_at, updated_at, deleted_at, uuid, username, password, nick_name, header_img, authority_id, phone, email, enable, origin_setting)
SELECT NOW(3), NOW(3), NULL, UUID(), x.un, '$2a$10$MEmG541pKYusIeo9Fs6JJeHACB775AVxGVxWbkYDoaTHPgp1ScbNa', x.nick, '', x.aid, '', '', 1, NULL
FROM (
  SELECT 'ops1' AS un, '运营·张敏' AS nick, 9001 AS aid
  UNION SELECT 'wh1', '仓库·李强', 9002
  UNION SELECT 'cs1', '客服·王芳', 9003
) x
WHERE NOT EXISTS (SELECT 1 FROM sys_users u WHERE u.username = x.un AND u.deleted_at IS NULL);

INSERT INTO sys_user_authority (sys_user_id, sys_authority_authority_id)
SELECT u.id, u.authority_id
FROM sys_users u
WHERE u.username IN ('ops1', 'wh1', 'cs1') AND u.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_user_authority ua
                  WHERE ua.sys_user_id = u.id AND ua.sys_authority_authority_id = u.authority_id);

-- 菜单授权：各自只看得到自己那一页（目录 shopAdmin + 对应子菜单）
INSERT INTO sys_authority_menus (sys_base_menu_id, sys_authority_authority_id)
SELECT m.id, 9001 FROM sys_base_menus m
WHERE m.name IN ('shopAdmin', 'shopOps') AND m.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_authority_menus am WHERE am.sys_base_menu_id = m.id AND am.sys_authority_authority_id = 9001);

INSERT INTO sys_authority_menus (sys_base_menu_id, sys_authority_authority_id)
SELECT m.id, 9002 FROM sys_base_menus m
WHERE m.name IN ('shopAdmin', 'shopWarehouse') AND m.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_authority_menus am WHERE am.sys_base_menu_id = m.id AND am.sys_authority_authority_id = 9002);

INSERT INTO sys_authority_menus (sys_base_menu_id, sys_authority_authority_id)
SELECT m.id, 9003 FROM sys_base_menus m
WHERE m.name IN ('shopAdmin', 'shopCs') AND m.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_authority_menus am WHERE am.sys_base_menu_id = m.id AND am.sys_authority_authority_id = 9003);

-- Casbin 最小权限：运营只能调 /shop/ops/**、仓库只能调 /shop/warehouse/**、客服只能调 /shop/cs/**
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', 9001, x.path, x.method
FROM (
  SELECT '/shop/ops/summary' AS path, 'GET' AS method
  UNION SELECT '/shop/ops/products', 'GET'
  UNION SELECT '/shop/ops/products/:id', 'PATCH'
  UNION SELECT '/shop/ops/orders', 'GET'
  UNION SELECT '/shop/ops/orders/:orderNo', 'GET'
  UNION SELECT '/shop/ops/notices', 'GET'
  UNION SELECT '/shop/ops/abnormals', 'GET'
  UNION SELECT '/shop/ops/ledger', 'GET'
  UNION SELECT '/shop/ops/trail', 'GET'
) x
WHERE NOT EXISTS (SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = '9001' AND c.v1 = x.path AND c.v2 = x.method);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', 9002, x.path, x.method
FROM (
  SELECT '/shop/warehouse/shipable' AS path, 'GET' AS method
  UNION SELECT '/shop/warehouse/ship', 'POST'
  UNION SELECT '/shop/warehouse/shipments', 'GET'
  UNION SELECT '/shop/warehouse/orders/:orderNo', 'GET'
) x
WHERE NOT EXISTS (SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = '9002' AND c.v1 = x.path AND c.v2 = x.method);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', 9003, x.path, x.method
FROM (
  SELECT '/shop/cs/refunds' AS path, 'GET' AS method
  UNION SELECT '/shop/cs/refunds/audit', 'POST'
  UNION SELECT '/shop/cs/refunds/retry', 'POST'
  UNION SELECT '/shop/cs/abnormals', 'GET'
  UNION SELECT '/shop/cs/abnormals/handle', 'POST'
  UNION SELECT '/shop/cs/notices', 'GET'
  UNION SELECT '/shop/cs/orders/:orderNo', 'GET'
  UNION SELECT '/shop/cs/channel/mode', 'POST'
) x
WHERE NOT EXISTS (SELECT 1 FROM casbin_rule c WHERE c.ptype = 'p' AND c.v0 = '9003' AND c.v1 = x.path AND c.v2 = x.method);
