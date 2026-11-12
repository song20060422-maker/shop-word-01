# 将 docs/events.md（GBK）事件表导出为 JSONL 结构化事件流（I-17）
# 用法: powershell -File export_sessions.ps1
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)  # docs/
$repo = Split-Path -Parent $root          # gin-vue-admin/
$src  = Join-Path $root 'events.md'
$outDir = Join-Path $root 'evidence\sessions'
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$lines = [System.IO.File]::ReadAllLines($src, [System.Text.Encoding]::UTF8)
$out = @()
$exportedAt = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss zzz')
foreach ($line in $lines) {
    if ($line -notmatch '^\|\s*(\d+)\s*\|') { continue }
    $cells = ($line -replace '^\|', '' -replace '\|$', '').Split('|') | ForEach-Object { $_.Trim() }
    if ($cells.Count -lt 6) { continue }
    $seq = 0
    if (-not [int]::TryParse($cells[0], [ref]$seq)) { continue }
    $obj = [ordered]@{
        seq         = $seq
        ts          = $cells[1]
        actor       = ($cells[2] -replace '`', '')
        event       = ($cells[3] -replace '`', '')
        output      = ($cells[4] -replace '`', '')
        task        = ($cells[5] -replace '`', '')
        source      = 'docs/events.md'
        exported_at = $exportedAt
    }
    $out += ($obj | ConvertTo-Json -Compress)
}
$outFile = Join-Path $outDir 'events-20261112.jsonl'
[System.IO.File]::WriteAllLines($outFile, $out, (New-Object System.Text.UTF8Encoding($false)))
Write-Output ("exported {0} events -> {1}" -f $out.Count, $outFile)
