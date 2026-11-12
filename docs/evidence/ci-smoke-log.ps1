$ErrorActionPreference = 'Stop'
$api = Join-Path $PSScriptRoot 'gh-api.ps1'
$bash = 'D:\download\Git\bin\bash.exe'
$credOut = & $bash -c "printf 'protocol=https\nhost=github.com\n' | git credential fill" 2>&1
$password = $null
foreach ($line in ($credOut -split "`n")) {
    if ($line -match '^password=(.+)$') { $password = $Matches[1].TrimEnd("`r") }
}
$headers = @{
    Authorization = "Bearer $password"
    'User-Agent'  = 'shop-word-remediation'
    Accept        = 'application/vnd.github+json'
}
$log = Invoke-WebRequest -Uri 'https://api.github.com/repos/song20060422-maker/shop-word-01/actions/jobs/131216343847/logs' -Headers $headers -MaximumRedirection 5 -UseBasicParsing
$text = $log.Content
$out = Join-Path $PSScriptRoot 'ci-run1-job.log'
[System.IO.File]::WriteAllText($out, $text, (New-Object System.Text.UTF8Encoding($false)))
# 输出冒烟步骤附近日志
$lines = $text -split "`n"
$start = -1
for ($i = 0; $i -lt $lines.Count; $i++) { if ($lines[$i] -match '\[12\]|\u5192\u70df') { $start = $i; break } }
if ($start -lt 0) { $start = [Math]::Max(0, $lines.Count - 80) }
$lines[[Math]::Max(0,$start-2)..[Math]::Min($lines.Count-1, $start+80)]
