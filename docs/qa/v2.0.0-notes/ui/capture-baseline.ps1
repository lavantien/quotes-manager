param(
    [string]$OutDir = "docs/qa/v2.0.0-notes/ui",
    [string]$Prefix = "01-baseline",
    [int]$Port = 8791
)
$ErrorActionPreference = "Stop"
$root = "C:\Users\lavantien\dev\github\quotes-manager"
$bin = Join-Path $root "dist\qm.exe"
$chrome = "C:\Program Files\Google\Chrome\Application\chrome.exe"
$runDir = Join-Path $root "dist\ui-capture"
$outAbs = Join-Path $root $OutDir

function Log($msg) { Write-Host "[capture] $msg" }

if (-not (Test-Path $bin)) { throw "missing $bin (run make build first)" }
if (-not (Test-Path $chrome)) { throw "missing chrome at $chrome" }
New-Item -ItemType Directory -Force -Path $outAbs | Out-Null
if (Test-Path $runDir) { Remove-Item -Recurse -Force $runDir }
New-Item -ItemType Directory -Force -Path $runDir | Out-Null

$canonSrc = Join-Path $root "data\e2e-live\db"
$dbDir = Join-Path $runDir "db"
New-Item -ItemType Directory -Force -Path $dbDir | Out-Null
$srcDb = Get-ChildItem $canonSrc -Filter *.db | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if (-not $srcDb) { throw "no canon db under $canonSrc" }
Copy-Item $srcDb.FullName (Join-Path $dbDir $srcDb.Name)
$runID = $srcDb.BaseName -replace '^canon-', ''
$pointer = '{{"current":"{0}","path":"{1}","updated":"{2}"}}' -f $runID, (($dbDir + "\" + $srcDb.Name) -replace '\\', '/'), (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
Set-Content -Path (Join-Path $dbDir "pointer.json") -Value $pointer -NoNewline
Log "canon db: $($srcDb.Name) run=$runID"

$env:QM_CANON_DB_DIR = $dbDir
$env:QM_SEED_FILE = Join-Path $root "database\seed.sql"
$serveLog = Join-Path $runDir "serve.log"
$server = Start-Process -FilePath $bin -ArgumentList @("serve", "--addr", "127.0.0.1:$Port", "--db", (Join-Path $runDir "quotes.db")) -WorkingDirectory $root -PassThru -WindowStyle Hidden -RedirectStandardOutput $serveLog -RedirectStandardError (Join-Path $runDir "serve.err.log")
Log "server pid=$($server.Id) on 127.0.0.1:$Port"

$base = "http://127.0.0.1:$Port"
$ready = $false
foreach ($i in 1..60) {
    if ($server.HasExited) { throw "server exited early, log: $(Get-Content $serveLog -Raw)" }
    try {
        $r = Invoke-WebRequest -Uri "$base/" -UseBasicParsing -TimeoutSec 2
        if ($r.StatusCode -eq 200) { $ready = $true; break }
    } catch { Start-Sleep -Milliseconds 400 }
}
if (-not $ready) { throw "server never became ready on $base" }
Log "server ready"

$shots = @(
    @{ n = "workspace"; u = "/"; w = 1440; h = 900 },
    @{ n = "workspace-1000"; u = "/"; w = 1000; h = 900 },
    @{ n = "workspace-720"; u = "/"; w = 720; h = 900 },
    @{ n = "dashboard"; u = "/dashboard"; w = 1440; h = 900 },
    @{ n = "texts"; u = "/texts"; w = 1440; h = 900 },
    @{ n = "text"; u = "/texts/mn1"; w = 1440; h = 900 },
    @{ n = "words"; u = "/words"; w = 1440; h = 900 },
    @{ n = "word"; u = "/words/dhamma"; w = 1440; h = 900 },
    @{ n = "graph"; u = "/graph"; w = 1440; h = 900 },
    @{ n = "workspace-375"; u = "/"; w = 375; h = 800 },
    @{ n = "workspace-scrollbars"; u = "/"; w = 1440; h = 900; scrollbars = $true }
)
$profile = [System.IO.Path]::GetFullPath((Join-Path $runDir "chrome-profile"))
New-Item -ItemType Directory -Force -Path $profile | Out-Null
$spawned = @()
foreach ($s in $shots) {
    $out = [System.IO.Path]::GetFullPath((Join-Path $outAbs ("{0}-{1}.png" -f $Prefix, $s.n)))
    $chromeArgs = @("--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check",
        "--window-size=$($s.w),$($s.h)", "--virtual-time-budget=9000", "--user-data-dir=$profile",
        "--screenshot=$out", "$base$($s.u)")
    if (-not $s.scrollbars) { $chromeArgs += "--hide-scrollbars" }
    $done = $false
    foreach ($attempt in 1..2) {
        $p = Start-Process -FilePath $chrome -ArgumentList $chromeArgs -PassThru -WindowStyle Hidden
        $spawned += $p
        if ($p.WaitForExit(120000)) { $done = $true; break }
        $p.Kill($true)
        Log "shot $($s.u) attempt $attempt timed out, retrying"
    }
    if (-not $done) { throw "chrome timed out on $($s.u) after retries" }
    if (-not (Test-Path $out)) { throw "screenshot missing for $($s.u)" }
    Log "shot $($s.u) -> $out ($((Get-Item $out).Length) bytes)"
}

foreach ($p in $spawned) { if (-not $p.HasExited) { $p.Kill($true) } }
if (-not $server.HasExited) { Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue }
$server.WaitForExit()
Start-Sleep -Milliseconds 500
foreach ($name in @("qm", "chrome")) {
    $left = Get-Process -Name $name -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -and ($_.Path -like "$root*" -or $_.Path -eq $chrome) }
    foreach ($lp in $left) {
        Log "cleanup leftover $($lp.Name) pid=$($lp.Id)"
        Stop-Process -Id $lp.Id -Force -ErrorAction SilentlyContinue
    }
}
Log "exit ok: $($shots.Count) shots in $outAbs"
