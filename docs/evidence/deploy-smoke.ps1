param([string]$Base = 'http://127.0.0.1:18099')
$ErrorActionPreference = 'Stop'
$exit = 0
function Step($ok, $msg) {
    if ($ok) { Write-Output ("[ OK ] " + $msg) } else { Write-Output ("[FAIL] " + $msg); $script:exit = 1 }
}
try {
    $h = Invoke-RestMethod -Uri "$Base/api/health" -Method GET -TimeoutSec 10
    Step ($h.status -eq 'ok') ("健康检查 status=" + $h.status)

    $login = Invoke-RestMethod -Uri "$Base/api/login" -Method POST -ContentType 'application/json' `
        -Body (@{ username = 'buyer1'; password = 'shop123' } | ConvertTo-Json) -TimeoutSec 10
    Step ($login.token -and $login.token.Length -gt 20) "买家登录（token 取得）"
    $tok = $login.token

    $ord = Invoke-RestMethod -Uri "$Base/api/orders" -Method POST -ContentType 'application/json' `
        -Headers @{ Authorization = "Bearer $tok" } `
        -Body (@{ product_id = 'SKU-A'; quantity = 1 } | ConvertTo-Json) -TimeoutSec 10
    Step ($ord.order_no -and $ord.amount -gt 0) ("下单 " + $ord.order_no + " 金额 " + $ord.amount + " 分")

    $det = Invoke-RestMethod -Uri ("$Base/api/orders/" + $ord.order_no) -Method GET `
        -Headers @{ Authorization = "Bearer $tok" } -TimeoutSec 10
    Step ($det.status -eq 'WAIT_PAY') ("订单详情状态=" + $det.status)

    $txn = "DEPLOY-TXN-" + (Get-Date -Format 'HHmmss')
    $body = '{"order_no":"' + $ord.order_no + '","channel_txn_no":"' + $txn + '","amount":' + $ord.amount + '}'
    $hmac = New-Object System.Security.Cryptography.HMACSHA256
    $hmac.Key = [System.Text.Encoding]::UTF8.GetBytes('channel-shared-secret')
    $sig = ($hmac.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($body)) | ForEach-Object { $_.ToString('x2') }) -join ''
    $notify = Invoke-RestMethod -Uri "$Base/api/pay/notify" -Method POST -ContentType 'application/json' `
        -Headers @{ 'X-Channel-Signature' = $sig } -Body $body -TimeoutSec 10
    Step ($notify.status -eq 'SUCCESS') ("支付通知受理=" + $notify.status)

    $det2 = Invoke-RestMethod -Uri ("$Base/api/orders/" + $ord.order_no) -Method GET `
        -Headers @{ Authorization = "Bearer $tok" } -TimeoutSec 10
    Step ($det2.status -eq 'TO_SHIP') ("支付后订单状态(已支付待发货)=" + $det2.status)

    Write-Output "部署冒烟通过：健康 -> 登录 -> 下单 -> 查单 -> 支付 -> 已支付"
} catch {
    Write-Output ("[FAIL] 异常: " + $_.Exception.Message)
    $exit = 1
}
exit $exit
