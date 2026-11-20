param([string]$PackagePath,[string]$InstallPath="$env:ProgramFiles/MyGittu",[switch]$Remove,[switch]$IfRegistered)
$ErrorActionPreference='Stop'
if($Remove){Get-AppxPackage -Name Oranjekop.MyGittu.Shell | Remove-AppxPackage;return}
if($IfRegistered -and !(Get-AppxPackage -Name Oranjekop.MyGittu.Shell)){return}
if(!$PackagePath){throw 'PackagePath is required for registration'}
if([Environment]::OSVersion.Version.Build -lt 22000){return}
$package=(Resolve-Path -LiteralPath $PackagePath).Path
$location=(Resolve-Path -LiteralPath $InstallPath).Path
if(!(Test-Path -LiteralPath "$location/desktop.exe") -or
   !(Get-ChildItem -LiteralPath $location -Filter 'MyGittuShell*.dll' -File)){throw 'Install MyGittu before registering its modern context menu'}
try {
    Add-AppxPackage -Path $package -ExternalLocation $location
} catch {
    # A loaded Explorer extension can prevent Windows from replacing the old registration.
    if ($_ -notmatch '(?i)0x80073d02' -or !(Get-AppxPackage -Name Oranjekop.MyGittu.Shell)) { throw }
    Add-AppxPackage -Path $package -ExternalLocation $location -ForceTargetApplicationShutdown
}
