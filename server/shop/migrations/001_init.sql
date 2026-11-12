-- ============================================================
-- 题目01 Shop 商城小型交易系统 · 初始化建表
-- 目标库：shop（MySQL 8.0）
-- 可重复执行：全部使用 IF NOT EXISTS
-- ============================================================

SET NAMES utf8mb4;

-- ---------- 账号 ----------
CREATE TABLE IF NOT EXISTS users (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username      VARCHAR(64)  NOT NULL,
  password_hash VARCHAR(160) NOT NULL DEFAULT '',
  role          ENUM('buyer','ops','warehouse','cs') NOT NULL,
  display_name  VARCHAR(64)  NOT NULL DEFAULT '',
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='买家与员工账号';

CREATE TABLE IF NOT EXISTS sessions (
  token      CHAR(64)        NOT NULL,
  user_id    BIGINT UNSIGNED NOT NULL,
  role       ENUM('buyer','ops','warehouse','cs') NOT NULL,
  expires_at DATETIME(3)     NOT NULL,
  created_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (token),
  KEY idx_sessions_user (user_id),
  KEY idx_sessions_expire (expires_at),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='登录会话';

-- ---------- 商品与库存 ----------
CREATE TABLE IF NOT EXISTS products (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  sku           VARCHAR(32)  NOT NULL,
  name          VARCHAR(128) NOT NULL,
  price_cents   BIGINT       NOT NULL COMMENT '单价，单位：分',
  available     INT          NOT NULL COMMENT '当前可售数量',
  initial_stock INT          NOT NULL COMMENT '初始库存，用于守恒校验',
  on_sale       TINYINT(1)   NOT NULL DEFAULT 1,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_products_sku (sku),
  CONSTRAINT ck_products_available CHECK (available >= 0),
  CONSTRAINT ck_products_price     CHECK (price_cents > 0),
  CONSTRAINT ck_products_initial   CHECK (initial_stock >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品与可售数量';

-- ---------- 订单 ----------
CREATE TABLE IF NOT EXISTS orders (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no       CHAR(20)     NOT NULL,
  buyer_id       BIGINT UNSIGNED NOT NULL,
  product_id     BIGINT UNSIGNED NOT NULL,
  product_name   VARCHAR(128) NOT NULL COMMENT '下单时商品名快照',
  price_cents    BIGINT       NOT NULL COMMENT '下单时单价快照',
  qty            INT          NOT NULL,
  amount_cents   BIGINT       NOT NULL COMMENT '金额快照 = price*qty',
  status         ENUM('pending_payment','cancelled','closed','paid','shipped',
                      'refund_requested','refund_failed','refunded') NOT NULL,
  expire_at      DATETIME(3)  NOT NULL COMMENT '服务端计算的付款截止时间',
  paid_at        DATETIME(3)  NULL,
  shipped_at     DATETIME(3)  NULL,
  closed_at      DATETIME(3)  NULL,
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_orders_no (order_no),
  KEY idx_orders_buyer (buyer_id, created_at),
  KEY idx_orders_status_expire (status, expire_at),
  CONSTRAINT fk_orders_buyer   FOREIGN KEY (buyer_id)   REFERENCES users(id),
  CONSTRAINT fk_orders_product FOREIGN KEY (product_id) REFERENCES products(id),
  CONSTRAINT ck_orders_qty    CHECK (qty BETWEEN 1 AND 2),
  CONSTRAINT ck_orders_amount CHECK (amount_cents > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单（含商品与价格快照）';

-- ---------- 支付与通知 ----------
CREATE TABLE IF NOT EXISTS payments (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no       CHAR(20)     NOT NULL,
  channel_txn_id VARCHAR(64)  NOT NULL COMMENT '渠道交易号',
  amount_cents   BIGINT       NOT NULL,
  paid_at        DATETIME(3)  NOT NULL,
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_payments_txn   (channel_txn_id) COMMENT '同一渠道交易号不得归属两个订单',
  UNIQUE KEY uk_payments_order (order_no)       COMMENT '一单只记款一次',
  CONSTRAINT fk_payments_order FOREIGN KEY (order_no) REFERENCES orders(order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='成功记款（幂等基石）';

CREATE TABLE IF NOT EXISTS payment_notices (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no       VARCHAR(32)  NOT NULL DEFAULT '',
  channel_txn_id VARCHAR(64)  NOT NULL DEFAULT '',
  amount_cents   BIGINT       NOT NULL DEFAULT 0,
  sign_ok        TINYINT(1)   NOT NULL DEFAULT 0,
  result         ENUM('accepted','rejected','duplicated','abnormal') NOT NULL,
  reason         VARCHAR(255) NOT NULL DEFAULT '',
  raw_body       TEXT         NULL,
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_notices_order  (order_no),
  KEY idx_notices_result (result, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付通知原始留痕（含被拒）';

-- ---------- 退款 ----------
CREATE TABLE IF NOT EXISTS refunds (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  refund_no      CHAR(20)     NOT NULL,
  order_no       CHAR(20)     NOT NULL,
  source         ENUM('presale','abnormal_late') NOT NULL COMMENT '售前退款/迟到支付退款',
  amount_cents   BIGINT       NOT NULL,
  audit_status   ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending',
  audit_reason   VARCHAR(255) NOT NULL DEFAULT '',
  audit_by       BIGINT UNSIGNED NULL,
  audit_at       DATETIME(3)  NULL,
  channel_status ENUM('pending','success','failed') NOT NULL DEFAULT 'pending',
  retry_count    INT          NOT NULL DEFAULT 0,
  restored       TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否已恢复可售数量',
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_refunds_no    (refund_no),
  UNIQUE KEY uk_refunds_order (order_no) COMMENT '整单退款，一单一张',
  KEY idx_refunds_audit (audit_status, channel_status),
  CONSTRAINT fk_refunds_order FOREIGN KEY (order_no) REFERENCES orders(order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='退款单';

-- ---------- 发货 ----------
CREATE TABLE IF NOT EXISTS shipments (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no    CHAR(20)     NOT NULL,
  carrier     VARCHAR(32)  NOT NULL,
  tracking_no VARCHAR(64)  NOT NULL,
  created_by  BIGINT UNSIGNED NULL,
  created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_shipments_order (order_no) COMMENT '合法订单只能登记一次发货',
  CONSTRAINT fk_shipments_order FOREIGN KEY (order_no) REFERENCES orders(order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='发货登记';

-- ---------- 异常支付 ----------
CREATE TABLE IF NOT EXISTS abnormal_payments (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no       VARCHAR(32)  NOT NULL,
  channel_txn_id VARCHAR(64)  NOT NULL,
  amount_cents   BIGINT       NOT NULL,
  reason         VARCHAR(255) NOT NULL DEFAULT '',
  status         ENUM('pending','handled') NOT NULL DEFAULT 'pending',
  handled_by     BIGINT UNSIGNED NULL,
  handled_at     DATETIME(3)  NULL,
  refund_id      BIGINT UNSIGNED NULL,
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_abnormal_txn (channel_txn_id),
  KEY idx_abnormal_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='迟到支付等异常收款待处理';

-- ---------- 库存流水与操作留痕 ----------
CREATE TABLE IF NOT EXISTS inventory_ledger (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  product_id      BIGINT UNSIGNED NOT NULL,
  order_no        VARCHAR(32)  NOT NULL DEFAULT '',
  change_type     ENUM('occupy','release','restore','ship_out') NOT NULL,
  delta           INT          NOT NULL COMMENT '对可售数量的增减（负数为扣减）',
  available_after INT          NOT NULL,
  created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_ledger_product (product_id, id),
  KEY idx_ledger_order   (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='库存变动流水（守恒校验依据）';

CREATE TABLE IF NOT EXISTS operation_logs (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_no   VARCHAR(32)  NULL,
  actor_id   BIGINT UNSIGNED NULL,
  actor_role VARCHAR(16)  NOT NULL DEFAULT '',
  action     VARCHAR(48)  NOT NULL,
  detail     VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_oplogs_order (order_no, id),
  KEY idx_oplogs_time  (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作留痕（追加写，不提供修改与删除接口）';
