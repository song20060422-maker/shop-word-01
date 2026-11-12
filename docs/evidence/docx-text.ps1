param([string]$DocxPath, [string]$OutTxt)
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [System.IO.Compression.ZipFile]::OpenRead($DocxPath)
$entry = $zip.GetEntry('word/document.xml')
$reader = New-Object System.IO.StreamReader($entry.Open(), [System.Text.Encoding]::UTF8)
$xml = $reader.ReadToEnd()
$reader.Close()
$zip.Dispose()
# 段落换行
$xml = $xml -replace '</w:p>', "`n"
# 去除其余标签
$text = $xml -replace '<[^>]+>', ''
$text = [System.Net.WebUtility]::HtmlDecode($text)
[System.IO.File]::WriteAllText($OutTxt, $text, (New-Object System.Text.UTF8Encoding($false)))
"lines: " + ($text -split "`n").Count
