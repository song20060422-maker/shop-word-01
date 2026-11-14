$ErrorActionPreference = 'Stop'
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
$jobs = Invoke-RestMethod -Uri 'https://api.github.com/repos/song20060422-maker/shop-word-01/actions/runs/43230918970/jobs' -Headers $headers
$jobId = $jobs.jobs[0].id
$log = Invoke-WebRequest -Uri "https://api.github.com/repos/song20060422-maker/shop-word-01/actions/jobs/$jobId/logs" -Headers $headers -UseBasicParsing
$out = Join-Path $PSScriptRoot 'ci-run2-job.log'
[System.IO.File]::WriteAllText($out, $log.Content, (New-Object System.Text.UTF8Encoding($false)))
"saved $out"
