param(
    [ValidateSet('Status','Menu','Modern','Overlays','Startup','Refresh','Maintain','Remove')]
    [string]$Action,
    [ValidateSet('On','Off')]
    [string]$State = 'Off',
    [switch]$Elevated
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$root = $PSScriptRoot
$classes = 'HKLM:\SOFTWARE\Classes'
$overlayRoot = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\ShellIconOverlayIdentifiers'
$machineRun = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run'
$userRun = 'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run'
$menuClasses = @('*','Directory','Directory\Background')
$overlayNames = @('MyGittu Clean','MyGittu Modified','MyGittu Conflict')

function ClassID([int]$Index) { '{29810E41-632A-4A9B-A190-4D659366810' + $Index + '}' }
function MenuPath([string]$Kind) { Join-Path $classes "$Kind\shellex\ContextMenuHandlers\MyGittu" }
function COMPath([int]$Index) { Join-Path $classes ('CLSID\' + (ClassID $Index)) }
function RegistryPath([string]$Path) { $Path.Substring('HKLM:\'.Length) }
function Write-Default([string]$Path,[string]$Value) {
    $key = [Microsoft.Win32.Registry]::LocalMachine.CreateSubKey((RegistryPath $Path))
    try { $key.SetValue('', $Value) } finally { $key.Dispose() }
}
function Read-Default([string]$Path) {
    $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey((RegistryPath $Path))
    if ($key) { try { $key.GetValue('') } finally { $key.Dispose() } }
}
function Remove-Key([string]$Path) {
    [Microsoft.Win32.Registry]::LocalMachine.DeleteSubKeyTree((RegistryPath $Path), $false)
}
function Run-Hive([string]$Path) {
    if ($Path.StartsWith('HKCU:')) { return [Microsoft.Win32.Registry]::CurrentUser }
    return [Microsoft.Win32.Registry]::LocalMachine
}
function Read-Run([string]$Path,[string]$Name) {
    $key = (Run-Hive $Path).OpenSubKey((RegistryPath $Path))
    if ($key) { try { $key.GetValue($Name) } finally { $key.Dispose() } }
}
function Set-Run([string]$Path,[string]$Name,[string]$Exe,[bool]$Enabled) {
    $hive = Run-Hive $Path
    $relative = RegistryPath $Path
    if ($Enabled) {
        $key = $hive.CreateSubKey($relative)
        try { $key.SetValue($Name, ('"' + (Join-Path $root $Exe) + '"')) } finally { $key.Dispose() }
    } else {
        $key = $hive.OpenSubKey($relative, $true)
        if ($key) { try { $key.DeleteValue($Name, $false) } finally { $key.Dispose() } }
    }
}
function Set-COM([int]$Index,[string]$Dll) {
    $path = Join-Path (COMPath $Index) 'InprocServer32'
    Write-Default $path $Dll
    New-ItemProperty -LiteralPath $path -Name ThreadingModel -Value Apartment -PropertyType String -Force | Out-Null
}
function MenuEnabled { (Read-Default (MenuPath 'Directory')) -eq (ClassID 1) }
function OverlaysEnabled { (Read-Default (Join-Path $overlayRoot $overlayNames[0])) -eq (ClassID 2) }

if ($Action -eq 'Status') {
    @{
        startup = [bool](Read-Run $userRun MyGittu)
        modern = [bool]([Environment]::OSVersion.Version.Build -ge 22000 -and (Get-AppxPackage -Name Oranjekop.MyGittu.Shell))
        modernAvailable = [bool](Test-Path -LiteralPath (Join-Path $root 'MyGittuShell.msix'))
        menu = [bool](MenuEnabled)
        overlays = [bool](OverlaysEnabled)
    } | ConvertTo-Json -Compress
    return
}
if ($Action -eq 'Startup') {
    Set-Run $userRun MyGittu desktop.exe ($State -eq 'On')
    return
}
if ($Action -eq 'Refresh') {
    & (Join-Path $root 'restart-explorer.ps1')
    return
}
if ($Action -eq 'Modern') {
    $package = Join-Path $root 'MyGittuShell.msix'
    if ($State -eq 'On' -and (Test-Path -LiteralPath $package)) {
        & (Join-Path $root 'register-modern.ps1') -PackagePath $package -InstallPath $root
    } elseif ($State -eq 'Off' -and (Test-Path -LiteralPath (Join-Path $root 'register-modern.ps1'))) {
        & (Join-Path $root 'register-modern.ps1') -Remove
    }
    return
}
if ($Action -eq 'Remove') {
    Set-Run $userRun MyGittu desktop.exe $false
    if (Test-Path -LiteralPath (Join-Path $root 'register-modern.ps1')) {
        try { & (Join-Path $root 'register-modern.ps1') -Remove } catch { Write-Warning $_.Exception.Message }
    }
    # Continue with machine-wide cleanup.
}
if (!$Elevated -and $Action -in 'Menu','Overlays') {
    $process = Start-Process -FilePath (Join-Path $PSHOME 'powershell.exe') -Verb RunAs -Wait -PassThru -WindowStyle Hidden -ArgumentList @(
        '-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',('"' + $PSCommandPath + '"'),
        '-Action',$Action,'-State',$State,'-Elevated')
    if ($process.ExitCode -ne 0) { throw "Explorer registration failed (exit $($process.ExitCode))." }
    if ($State -eq 'On' -and $Action -eq 'Overlays') {
        Start-Process -FilePath (Join-Path $root 'agent.exe') -WindowStyle Hidden
    }
    if ($Action -eq 'Menu' -and [Environment]::OSVersion.Version.Build -ge 22000) {
        & $PSCommandPath -Action Modern -State $State
    }
    return
}
if ($Action -in 'Menu','Overlays','Maintain' -and ($State -eq 'On' -or $Action -eq 'Maintain')) {
    $dll = Get-ChildItem -LiteralPath $root -Filter 'MyGittuShell-*.dll' -File |
        Where-Object { $_.BaseName -match '^MyGittuShell-[0-9]+\.[0-9]+\.[0-9]+$' } |
        Sort-Object { [version]($_.BaseName -replace '^MyGittuShell-','') } -Descending | Select-Object -First 1
    if (!$dll) { throw 'MyGittu Explorer DLL is missing.' }
}
if ($Action -eq 'Menu') {
    if ($State -eq 'On') {
        Set-COM 1 $dll.FullName
        foreach ($kind in $menuClasses) { Write-Default (MenuPath $kind) (ClassID 1) }
    } else {
        foreach ($kind in $menuClasses) { Remove-Key (MenuPath $kind) }
        Remove-Key (COMPath 1)
    }
}
if ($Action -eq 'Overlays') {
    for ($i=0; $i -lt $overlayNames.Count; $i++) {
        $index = $i + 2
        $path = Join-Path $overlayRoot $overlayNames[$i]
        if ($State -eq 'On') {
            Set-COM $index $dll.FullName
            Write-Default $path (ClassID $index)
        } else {
            Remove-Key $path
            Remove-Key (COMPath $index)
        }
    }
    Set-Run $machineRun MyGittuAgent agent.exe ($State -eq 'On')
}
if ($Action -eq 'Maintain') {
    if (MenuEnabled) { Set-COM 1 $dll.FullName }
    if (OverlaysEnabled) {
        for ($i=2; $i -le 4; $i++) { Set-COM $i $dll.FullName }
        Set-Run $machineRun MyGittuAgent agent.exe $true
    } else { Set-Run $machineRun MyGittuAgent agent.exe $false }
}
if ($Action -eq 'Remove') {
    foreach ($kind in $menuClasses) { Remove-Key (MenuPath $kind) }
    foreach ($name in $overlayNames) { Remove-Key (Join-Path $overlayRoot $name) }
    for ($i=1; $i -le 4; $i++) { Remove-Key (COMPath $i) }
    Set-Run $machineRun MyGittuAgent agent.exe $false
}
