$log = 'C:\Users\lavantien\dev\github\quotes-manager\docs\qa\logs\wave-final-verify.log'
$jobwrap = 'C:\Users\lavantien\dev\github\quotes-manager\dist\tools\jobwrap.exe'
$dev = 'C:\Users\lavantien\dev\github\quotes-manager\dist\tools\dev.exe'
$light = 'internal/canon/norm,internal/canon/source/editions,internal/canon/source/dpd,internal/canon/triangulate,internal/canon/server,internal/canon/jobs,internal/canon/analytics,internal/canon/export,internal/quote,internal/search,internal/store,internal/server,internal/cli'
"=== detached resume start $(Get-Date -Format o) ===" | Add-Content $log
& $jobwrap -jobmem 8589934592 $dev mutate -pkgs $light -parallel 4 2>&1 | Add-Content $log
"=== detached mutate-light exit $LASTEXITCODE $(Get-Date -Format o) ===" | Add-Content $log
if ($LASTEXITCODE -eq 0) {
  & $jobwrap -jobmem 6442450944 $dev mutate -pkgs internal/canon/source/bilara -parallel 1 2>&1 | Add-Content $log
  "=== detached mutate-bilara exit $LASTEXITCODE $(Get-Date -Format o) ===" | Add-Content $log
}
if ($LASTEXITCODE -eq 0) {
  & $jobwrap -jobmem 8589934592 $dev e2e-hermetic 2>&1 | Add-Content $log
  "=== detached e2e-hermetic exit $LASTEXITCODE $(Get-Date -Format o) ===" | Add-Content $log
}
"=== detached resume done $(Get-Date -Format o) ===" | Add-Content $log
