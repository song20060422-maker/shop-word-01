# 按包聚合覆盖率：单次合并 profile，每包并集口径（块级去重）
$prof = 'D:\Users\Lenovo\Desktop\shop_word\gin-vue-admin\server\coverage-single.out'
$pkgs = @{}
Get-Content $prof | Where-Object { $_ -match '^github' } | ForEach-Object {
    if ($_ -match '^(.+?\.go):\d+\.\d+,\d+\.\d+ \d+ (\d+)$') {
        $file = $Matches[1]; $count = [int]$Matches[2]
        if ($file -match '\\shop\\([a-z_]+)\\' -or $file -match '/shop/([a-z_]+)/') {
            $pkg = $Matches[1]
            if (-not $pkgs.ContainsKey($pkg)) { $pkgs[$pkg] = @{total=0; hit=0} }
            $pkgs[$pkg].total++
            if ($count -gt 0) { $pkgs[$pkg].hit++ }
        }
    }
}
$pkgs.GetEnumerator() | Sort-Object Key | ForEach-Object {
    $p = $_.Value; $pct = if ($p.total -gt 0) { [Math]::Round(100.0 * $p.hit / $p.total, 1) } else { 0 }
    "{0,-15} {1,6:P1}  ({2}/{3} blocks)" -f $_.Key, ($p.hit / [Math]::Max($p.total,1)), $p.hit, $p.total
}
