# GitHub API helper: fetch token via git credential fill (through Git bash), call GitHub API. No token echo.
param(
    [string]$Path = '/repos/song20060422-maker/shop-word-01/actions/runs?per_page=10',
    [string]$Method = 'GET',
    [object]$Body = $null,
    [switch]$SaveRaw,
    [string]$OutFile
)
$ErrorActionPreference = 'Stop'

$bash = 'D:\download\Git\bin\bash.exe'
$credOut = & $bash -c "printf 'protocol=https\nhost=github.com\n' | git credential fill" 2>&1
$password = $null
foreach ($line in ($credOut -split "`n")) {
    if ($line -match '^password=(.+)$') { $password = $Matches[1].TrimEnd("`r") }
}
if (-not $password) { throw 'git credential fill failed' }

$headers = @{
    Authorization          = "Bearer $password"
    'X-GitHub-Api-Version' = '2022-11-28'
    Accept                 = 'application/vnd.github+json'
    'User-Agent'           = 'shop-word-remediation'
}
$uri = "https://api.github.com$Path"
if ($Body) {
    $res = Invoke-RestMethod -Uri $uri -Headers $headers -Method $Method -ContentType 'application/json' -Body ($Body | ConvertTo-Json -Depth 10)
} else {
    $res = Invoke-RestMethod -Uri $uri -Headers $headers -Method $Method
}
if ($SaveRaw -and $OutFile) {
    $res | ConvertTo-Json -Depth 10 | Out-File -FilePath $OutFile -Encoding utf8
} else {
    $res | ConvertTo-Json -Depth 10
}
