param([Parameter(Mandatory=$true)][string]$PackagePath,[string]$InstallPath="$env:ProgramFiles/MyGittu",[switch]$Remove)
$ErrorActionPreference='Stop'
if($Remove){Get-AppxPackage -Name Oranjekop.MyGittu.Shell | Remove-AppxPackage;return}
$package=(Resolve-Path -LiteralPath $PackagePath).Path
$location=(Resolve-Path -LiteralPath $InstallPath).Path
if(!(Test-Path -LiteralPath "$location/MyGittuShell.dll") -or !(Test-Path -LiteralPath "$location/desktop.exe")){throw 'Install MyGittu before registering its modern context menu'}
Add-AppxPackage -Path $package -ExternalLocation $location
