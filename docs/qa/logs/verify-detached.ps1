$log = 'C:\Users\lavantien\dev\github\quotes-manager\docs\qa\logs\wave-final-verify.log'
$jobwrap = 'C:\Users\lavantien\dev\github\quotes-manager\dist\tools\jobwrap.exe'
$dev = 'C:\Users\lavantien\dev\github\quotes-manager\dist\tools\dev.exe'
"=== detached resume start $(Get-Date -Format o) ===" | Add-Content $log
& $jobwrap -jobmem 6442450944 $dev mutate 2>&1 | Add-Content $log
"=== detached mutate exit $LASTEXITCODE $(Get-Date -Format o) ===" | Add-Content $log
if ($LASTEXITCODE -eq 0) {
  & $jobwrap -jobmem 6442450944 $dev e2e-hermetic 2>&1 | Add-Content $log
  "=== detached e2e-hermetic exit $LASTEXITCODE $(Get-Date -Format o) ===" | Add-Content $log
}
"=== detached resume done $(Get-Date -Format o) ===" | Add-Content $log
