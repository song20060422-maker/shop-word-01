# Shop 业务冒烟（Windows 开发态）：健康检查 -> 登录 -> 下单 -> 查单
# 用法：powershell -ExecutionPolicy Bypass -File server\scripts\shop-smoke.ps1
param(
    [string]$Base = "http://127.0.0.1:8099",
    [string]$Password = "shop123"
)

$ErrorActionPreference = "Stop"

function Assert-Equal($expect, $actual, $msg) {
    if ($expect -ne $actual) { throw "[FAIL] $msg（期望 $expect 实际 $actual）" }
    Write-Host "[ OK ] $msg"
}

$health = Invoke-RestMethod -Uri "$Base/api/health" -Method Get
Assert-Equal "ok" $health.status "健康检查"

$login = Invoke-RestMethod -Uri "$Base/api/login" -Method Post -ContentType "application/json" `
    -Body (@{ username = "buyer1"; password = $Password } | ConvertTo-Json)
if (-not $login.token) { throw "[FAIL] 登录未返回 token" }
Write-Host "[ OK ] 买家登录"

$headers = @{ Authorization = "Bearer $($login.token)" }
$order = Invoke-RestMethod -Uri "$Base/api/orders" -Method Post -Headers $headers -ContentType "application/json" `
    -Body (@{ product_id = "SKU-A"; quantity = 1 } | ConvertTo-Json)
if (-not $order.order_no) { throw "[FAIL] 下单失败" }
Write-Host "[ OK ] 下单 $($order.order_no) 金额 $($order.amount) 分"

$detail = Invoke-RestMethod -Uri "$Base/api/orders/$($order.order_no)" -Method Get -Headers $headers
Assert-Equal "WAIT_PAY" $detail.status "订单详情状态"

Write-Host "冒烟通过：健康检查 -> 登录 -> 下单 -> 查单"
