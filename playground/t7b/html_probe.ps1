$ErrorActionPreference = 'Stop'
$base = 'http://127.0.0.1:8093'
$env:QM_QUOTES_DB_PATH = 'playground/t7b/live.db'
$env:QM_SERVER_ADDR = '127.0.0.1:8093'
$srv = Start-Process -FilePath (Resolve-Path dist\qm.exe) -ArgumentList 'serve' -RedirectStandardOutput playground\t7b\serve2.out.log -RedirectStandardError playground\t7b\serve2.err.log -PassThru -WindowStyle Hidden
try {
  $ready = $false
  for ($i = 0; $i -lt 40; $i++) {
    if ($srv.HasExited) { break }
    try { $r = Invoke-WebRequest "$base/" -UseBasicParsing -TimeoutSec 2; if ($r.StatusCode -eq 200) { $ready = $true; break } } catch {}
    Start-Sleep -Milliseconds 500
  }
  if (-not $ready) { throw "server not ready" }
  $html = (Invoke-WebRequest "$base/collections/1" -UseBasicParsing).Content
  $sections = ([regex]::Matches($html, 'class="collection-group"')).Count
  $heads = [regex]::Matches($html, 'id="group-(\d+)"') | ForEach-Object { $_.Groups[1].Value }
  "sections=$sections ids=$($heads -join ',')"
  $order = [regex]::Matches($html, 'id="group-[^"]*"') | ForEach-Object { $_.Value }
  "order=$($order -join ' ')"
  $nameLen = Invoke-WebRequest "$base/collections/1/groups" -Method Post -Body @{ name = ('x' * 5000) } -UseBasicParsing -SkipHttpErrorCheck
  "create-5000-chars: $($nameLen.StatusCode)"
  if ($nameLen.StatusCode -eq 200) {
    $html2 = (Invoke-WebRequest "$base/collections/1" -UseBasicParsing).Content
    "stored-long-name=$($html2.Contains('xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'))"
    $del = Invoke-WebRequest "$base/collections/1/groups/19" -Method Delete -UseBasicParsing -SkipHttpErrorCheck
    "cleanup-delete-19: $($del.StatusCode)"
  }
} finally {
  if (-not $srv.HasExited) { Stop-Process -Id $srv.Id -Force -Confirm:$false }
  Start-Sleep -Milliseconds 800
  $listen = Get-NetTCPConnection -LocalPort 8093 -State Listen -ErrorAction SilentlyContinue
  "server-stopped=$($null -eq $listen)"
}
