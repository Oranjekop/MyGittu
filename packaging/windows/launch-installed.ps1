param(
    [Parameter(Mandatory=$true)][string]$InstallPath,
    [Parameter(Mandatory=$true)][ValidateSet('Launch','Maintain')][string]$Action
)

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath($InstallPath)

# Resolve the desktop's automation object so launches use Explorer's user token.
$shell = New-Object -ComObject Shell.Application
$location = 0
$rootLocation = 0
$window = 0
$desktop = $shell.Windows().FindWindowSW([ref]$location, [ref]$rootLocation, 8, [ref]$window, 1)
if (!$desktop) { throw 'The Explorer desktop is unavailable.' }
$application = $desktop.Document.Application
if ($Action -eq 'Launch') {
    $application.ShellExecute((Join-Path $root 'desktop.exe'), '', $root, 'open', 1)
} else {
    $script = Join-Path $root 'integration.ps1'
    $arguments = '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' + $script + '" -Action ResumeIntegration'
    $application.ShellExecute((Join-Path $PSHOME 'powershell.exe'), $arguments, $root, 'open', 0)
}
