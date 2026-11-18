$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/restart-explorer.ps1"

class FakeExplorer {
    [string]$Path
    [int]$SessionId = 1
    [bool]$HasExited = $false
    [void] Kill() { $this.HasExited = $true }
    [bool] WaitForExit([int]$milliseconds) { return $this.HasExited }
}

# The only processes used by this test are in-memory fixtures.
function Get-Process([int]$Id) { $script:fakeShell }
function Get-ShellProcessId {
    if (!$script:fakeShell.HasExited) { return 111 }
    if ($script:autoRestart -or $script:started) { return 222 }
    return 0
}
function Start-Process([string]$FilePath) {
    if ($FilePath -ne (Join-Path $env:WINDIR 'explorer.exe')) {
        throw 'Unexpected shell executable'
    }
    $script:started = $true
}

$script:fakeShell = [FakeExplorer]::new()
$script:fakeShell.Path = Join-Path $env:WINDIR 'explorer.exe'
$script:autoRestart = $true
$script:started = $false
Restart-DesktopShell
if (!$fakeShell.HasExited -or $started) {
    throw 'The automatic shell relaunch should not open a second Explorer'
}

$script:fakeShell.HasExited = $false
$script:autoRestart = $false
Restart-DesktopShell
if (!$fakeShell.HasExited -or !$started) {
    throw 'The fallback must start Explorer when the desktop shell does not return'
}

$script:fakeShell.HasExited = $false
$script:fakeShell.Path = Join-Path $env:WINDIR 'another-shell.exe'
$rejected = $false
try { Restart-DesktopShell } catch { $rejected = $true }
if (!$rejected -or $fakeShell.HasExited) {
    throw 'A non-Explorer shell must not be stopped'
}
Write-Host 'Explorer restart checks passed'
