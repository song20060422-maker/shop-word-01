$ErrorActionPreference = 'Stop'
$api = Join-Path $PSScriptRoot 'gh-api.ps1'
$jobs = & $api -Path '/repos/song20060422-maker/shop-word-01/actions/runs/43228066959/jobs' | ConvertFrom-Json
foreach ($j in $jobs.jobs) {
    "JOB: {0} / {1}" -f $j.status, $j.conclusion
    foreach ($s in $j.steps) {
        "  {0}. {1} -> {2}" -f $s.number, $s.name, $s.conclusion
    }
}
