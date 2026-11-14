$ErrorActionPreference = 'Stop'
$bash = 'D:\download\Git\bin\bash.exe'
$credOut = & $bash -c "printf 'protocol=https\nhost=github.com\n' | git credential fill" 2>&1
$password = $null
foreach ($line in ($credOut -split "`n")) {
    if ($line -match '^password=(.+)$') { $password = $Matches[1].TrimEnd("`r") }
}
if (-not $password) { throw 'no token' }
$headers = @{
    Authorization          = "Bearer $password"
    'X-GitHub-Api-Version' = '2022-11-28'
    Accept                 = 'application/vnd.github+json'
    'User-Agent'           = 'shop-word-remediation'
}
$body = [System.Text.Encoding]::UTF8.GetBytes('{"private": false}')
$res = Invoke-RestMethod -Uri 'https://api.github.com/repos/song20060422-maker/shop-word-01' `
    -Headers $headers -Method PATCH -ContentType 'application/json' -Body $body
"visibility: private=$($res.private)"
