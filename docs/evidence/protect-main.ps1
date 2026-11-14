$ErrorActionPreference = 'Stop'
$bash = 'D:\download\Git\bin\bash.exe'
$credOut = & $bash -c "printf 'protocol=https\nhost=github.com\n' | git credential fill" 2>&1
$password = $null
foreach ($line in ($credOut -split "`n")) {
    if ($line -match '^password=(.+)$') { $password = $Matches[1].TrimEnd("`r") }
}
if (-not $password) { throw 'no token' }

# context: 质量门禁（gofmt / vet / build / test / 覆盖率 / race） as JSON unicode escapes
$context = '"\u8d28\u91cf\u95e8\u7981\uff08gofmt / vet / build / test / \u8986\u76d6\u7387 / race\uff09"'
$payloadRaw = '{"required_status_checks":{"strict":true,"contexts":[' + $context + ']},"enforce_admins":false,"required_pull_request_reviews":null,"restrictions":null,"allow_force_pushes":false,"allow_deletions":false,"required_conversation_resolution":false}'
$jsonBytes = [System.Text.Encoding]::UTF8.GetBytes($payloadRaw)
$headers = @{
    Authorization          = "Bearer $password"
    'X-GitHub-Api-Version' = '2022-11-28'
    Accept                 = 'application/vnd.github+json'
    'User-Agent'           = 'shop-word-remediation'
}
$res = Invoke-RestMethod -Uri 'https://api.github.com/repos/song20060422-maker/shop-word-01/branches/main/protection' `
    -Headers $headers -Method PUT -ContentType 'application/json' -Body $jsonBytes
$ctx = $res.required_status_checks.contexts -join ','
Write-Output ("protection set. strict=" + $res.required_status_checks.strict + " force_push_allowed=" + (-not $res.allow_force_pushes.enabled))
Write-Output ("contexts=" + $ctx)
