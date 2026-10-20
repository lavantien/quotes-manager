$ErrorActionPreference = 'Stop'
$base = 'http://127.0.0.1:8093'
$env:QM_QUOTES_DB_PATH = 'playground/t7b/live.db'
$env:QM_SERVER_ADDR = '127.0.0.1:8093'
$srv = Start-Process -FilePath (Resolve-Path dist\qm.exe) -ArgumentList 'serve' -RedirectStandardOutput playground\t7b\serve.out.log -RedirectStandardError playground\t7b\serve.err.log -PassThru -WindowStyle Hidden
try {
  $ready = $false
  for ($i = 0; $i -lt 40; $i++) {
    if ($srv.HasExited) { break }
    try { $r = Invoke-WebRequest "$base/" -UseBasicParsing -TimeoutSec 2; if ($r.StatusCode -eq 200) { $ready = $true; break } } catch {}
    Start-Sleep -Milliseconds 500
  }
  if (-not $ready) { throw "server not ready" }
  "ready pid=$($srv.Id)"

  function Post($path, $body) {
    Invoke-WebRequest "$base$path" -Method Post -Body $body -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
  }
  function Delete($path) {
    Invoke-WebRequest "$base$path" -Method Delete -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
  }

  $r = Post '/collections/1/groups' @{ name = 'Reviewer Probe' }
  "create: $($r.StatusCode)"
  $r = Post '/collections/1/groups' @{ name = '' }
  "create-empty: $($r.StatusCode)"
  $r = Post '/collections/1/groups' @{ name = "bad%00name" }
  "create-nul-form: $($r.StatusCode)"
  $r = Invoke-WebRequest "$base/collections/1/groups" -Method Post -Body @{ name = [char]7 + 'x' } -UseBasicParsing -SkipHttpErrorCheck
  "create-bel: $($r.StatusCode)"
  $r = Invoke-WebRequest "$base/collections/1/groups/17/rename" -Method Post -Headers @{ 'HX-Prompt' = [uri]::EscapeDataString('Probe Renamed') } -UseBasicParsing -SkipHttpErrorCheck
  "rename-prompt: $($r.StatusCode)"
  $r = Post '/collections/1/groups/17/move?dir=up' @{}
  "move-up: $($r.StatusCode)"
  $r = Post '/collections/1/groups/17/move?dir=down' @{}
  "move-down: $($r.StatusCode)"
  $r = Post '/collections/1/suttas' @{ muid = 'MN 26' }
  "add-existing: $($r.StatusCode)"
  $r = Post '/collections/1/suttas' @{ muid = 'mn999' }
  "add-mn999: $($r.StatusCode)"
  $r = Post '/collections/1/suttas' @{ muid = 'zz9' }
  "add-bad-family: $($r.StatusCode)"
  $r = Post '/collections/1/suttas/mn999/group' @{ group = '17' }
  "assign-mn999: $($r.StatusCode)"
  $r = Post '/collections/1/suttas/mn999/group' @{ group = '9999' }
  "assign-missing-group: $($r.StatusCode)"
  $r = Post '/collections/2/groups' @{ name = 'Cross Collection' }
  "create-missing-cid: $($r.StatusCode)"

  $ids = Invoke-WebRequest "$base/collections/1/ids.txt" -UseBasicParsing
  $idsLines = $ids.Content -split "`n" | Where-Object { $_.Trim() }
  "ids-lines=$($idsLines.Count) first=$($idsLines[0]) last=$($idsLines[-1])"
  "ids-has-mn999=$($idsLines -contains 'mn999') ids-sorted=$(@($idsLines -eq ($idsLines | Sort-Object)) -contains $true)"

  $html = Invoke-WebRequest "$base/collections/1" -UseBasicParsing
  $markers = @('Reviewer Probe', 'Probe Renamed', 'Five Daily Reflections', 'Striving for Celibacy', 'mn999', 'Ungrouped', 'KN Dhp 210')
  foreach ($m in $markers) { "html-has[$m]=$($html.Content.Contains($m))" }
  $groupCount = ([regex]::Matches($html.Content, 'group-heading')).Count
  "html-group-headings=$groupCount"

  $r = Delete '/collections/1/suttas/mn999'
  "remove-mn999: $($r.StatusCode)"
  $r = Delete '/collections/1/groups/17'
  "delete-group: $($r.StatusCode)"
  $r = Post '/collections/1/groups/17/rename' @{ name = 'Zombie' }
  "rename-deleted: $($r.StatusCode)"
} finally {
  if (-not $srv.HasExited) { Stop-Process -Id $srv.Id -Force -Confirm:$false }
  Start-Sleep -Milliseconds 800
  $listen = Get-NetTCPConnection -LocalPort 8093 -State Listen -ErrorAction SilentlyContinue
  "server-stopped=$($null -eq $listen)"
}
