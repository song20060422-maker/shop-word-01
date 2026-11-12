#!/usr/bin/env bash
# Shop 业务冒烟脚本（CI 用）：登录 -> 取商品 -> 下单 -> 查询订单 -> 支付通知。
# 依赖 curl + python3（解析 JSON）。退出码非 0 即冒烟失败。
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:8099}"
PASS="${SHOP_DEMO_PASSWORD:-shop123}"

require() {
  local expect="$1" actual="$2" msg="$3"
  if [ "$expect" != "$actual" ]; then
    echo "[FAIL] $msg (期望 $expect 实际 $actual)"
    exit 1
  fi
  echo "[ OK ] $msg"
}

TOKEN=$(curl -fsS -X POST "$BASE/api/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"buyer1\",\"password\":\"$PASS\"}" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
[ -n "$TOKEN" ] || { echo "[FAIL] 登录未返回 token"; exit 1; }
echo "[ OK ] 买家登录"

HEALTH=$(curl -fsS "$BASE/api/health" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')
require ok "$HEALTH" "健康检查"

ORDER=$(curl -fsS -X POST "$BASE/api/orders" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"product_id":"SKU-A","quantity":1}')
ORDER_NO=$(echo "$ORDER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["order_no"])')
AMOUNT=$(echo "$ORDER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["amount"])')
[ -n "$ORDER_NO" ] || { echo "[FAIL] 下单失败: $ORDER"; exit 1; }
echo "[ OK ] 下单 $ORDER_NO 金额 $AMOUNT 分"

DETAIL=$(curl -fsS "$BASE/api/orders/$ORDER_NO" -H "Authorization: Bearer $TOKEN" |
  python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["status"])')
require WAIT_PAY "$DETAIL" "订单详情状态"

# 支付通知走契约通道：对原始请求体做 HMAC-SHA256 签名
BODY="{\"order_no\":\"$ORDER_NO\",\"channel_txn_no\":\"CI-TXN-1\",\"amount\":$AMOUNT}"
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "${SHOP_PAY_SECRET:-channel-shared-secret}" -r | awk '{print $1}')
NOTIFY=$(curl -fsS -X POST "$BASE/api/pay/notify" \
  -H 'Content-Type: application/json' -H "X-Channel-Signature: $SIG" -d "$BODY")
RESULT=$(echo "$NOTIFY" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')
require SUCCESS "$RESULT" "支付通知受理"

echo "冒烟通过：登录 -> 健康检查 -> 下单 -> 查单 -> 支付"
