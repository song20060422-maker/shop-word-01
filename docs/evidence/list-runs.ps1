$ErrorActionPreference = 'Stop'
$api = Join-Path $PSScriptRoot 'gh-api.ps1'
$r = & $api -Path '/repos/song20060422-maker/shop-word-01/actions/runs?per_page=8' | ConvertFrom-Json
foreach ($run in $r.workflow_runs) {
    "{0} | run #{1} | {2} | {3} | sha={4} | {5}" -f $run.created_at, $run.run_number, $run.status, $run.conclusion, $run.head_sha.Substring(0,7), $run.html_url
}
