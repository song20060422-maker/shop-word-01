$ErrorActionPreference = 'Stop'
$api = Join-Path $PSScriptRoot 'gh-api.ps1'
$runId = $args[0]
if (-not $runId) { $runId = '43230918970' }
for ($i = 0; $i -lt 60; $i++) {
    $r = & $api -Path "/repos/song20060422-maker/shop-word-01/actions/runs/$runId" | ConvertFrom-Json
    if ($r.status -eq 'completed') {
        "run #{0} completed: {1} | sha={2}" -f $r.run_number, $r.conclusion, $r.head_sha.Substring(0,7)
        $jobs = & $api -Path "/repos/song20060422-maker/shop-word-01/actions/runs/$runId/jobs" | ConvertFrom-Json
        foreach ($j in $jobs.jobs) {
            "JOB {0}: {1} / {2}" -f $j.name, $j.status, $j.conclusion
            foreach ($s in $j.steps) { "  {0}. {1} -> {2}" -f $s.number, $s.name, $s.conclusion }
        }
        exit 0
    }
    Start-Sleep -Seconds 30
}
'timeout: run still in progress after 30min'
exit 1
