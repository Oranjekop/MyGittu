param([Parameter(Mandatory=$true)][string]$Installer,[string]$Version=(Get-Content "$PSScriptRoot/../../VERSION" -Raw).Trim())
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../.work/install-smoke'))
$installerPath=(Resolve-Path $Installer).Path
$uninstallKey='HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\{8E775DC6-289F-4CA1-9058-985C56602F39}_is1'
if(Test-Path -LiteralPath $uninstallKey){throw 'The smoke test requires a runner without an existing MyGittu installation'}
$installArgs=@('/S',('/D='+$root))
$process=Start-Process -FilePath $installerPath -ArgumentList $installArgs -Wait -PassThru
if($process.ExitCode -ne 0){throw "Installer exited $($process.ExitCode)"}
$agent=$null
$desktop=$null
$userFile=Join-Path $root 'smoke-user-file.txt'
$checkUserFile=$false
try{
 $file=Join-Path $root 'desktop.exe'
 $bytes=[IO.File]::ReadAllBytes($file);$pe=[BitConverter]::ToInt32($bytes,0x3c);$machine=[BitConverter]::ToUInt16($bytes,$pe+4)
 $native=[Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
 $expected=if($native -eq 'Arm64'){0xAA64}else{0x8664}
 if($machine -ne $expected){throw "Incorrect installed architecture: $machine for $native"}
 $agent=Start-Process -FilePath (Join-Path $root 'agent.exe') -WindowStyle Hidden -PassThru
 Start-Sleep -Seconds 1
 $health=(& (Join-Path $root 'mygittu.exe') -method health.ping)|ConvertFrom-Json
 if($LASTEXITCODE -or $health.app_version -ne $Version){throw 'Installed Agent health/version check failed'}
 if(!(Test-Path -LiteralPath (Join-Path $root "MyGittuShell-$Version.dll"))){throw 'Versioned shell DLL is missing'}
 $desktop=Start-Process -FilePath $file -PassThru
 Start-Sleep -Seconds 2
 if($desktop.HasExited){throw 'Installed desktop exited before the upgrade check'}
 # Simulate the old Inno uninstall entry in this isolated directory.
 $legacyUninstaller=Join-Path $root 'unins000.exe'
 New-Item -ItemType File -Path $legacyUninstaller | Out-Null
 Set-Content -LiteralPath $userFile -Value 'Keep during uninstall'
 $checkUserFile=$true
 Set-ItemProperty -LiteralPath $uninstallKey -Name UninstallString -Value ('"'+$legacyUninstaller+'"')
 Set-ItemProperty -LiteralPath $uninstallKey -Name InstallLocation -Value ($root+'\')
 $upgrade=Start-Process -FilePath $installerPath -ArgumentList '/S' -Wait -PassThru
 if($upgrade.ExitCode -ne 0 -or !$agent.HasExited -or !$desktop.HasExited){throw 'Installation over a running desktop and Agent failed'}
 $entry=Get-ItemProperty -LiteralPath $uninstallKey
 if($entry.InstallLocation -ne $root -or $entry.UninstallString -ne ('"'+(Join-Path $root 'Uninstall.exe')+'"') -or (Test-Path -LiteralPath $legacyUninstaller)){
  throw 'The installer did not take over the legacy uninstall entry'
 }
 Write-Host "Installed native $native package and verified Agent $Version"
}finally{
 if($desktop -and !$desktop.HasExited){Stop-Process -Id $desktop.Id -Force}
 if($agent -and !$agent.HasExited){Stop-Process -Id $agent.Id -Force}
 $uninstaller=Join-Path $root 'Uninstall.exe'
 if(Test-Path -LiteralPath $uninstaller){
  $removed=Start-Process -FilePath $uninstaller -ArgumentList '/S' -Wait -PassThru
  if($removed.ExitCode -ne 0 -or (Test-Path -LiteralPath (Join-Path $root 'desktop.exe')) -or (Test-Path -LiteralPath $uninstallKey)){throw 'Uninstaller failed'}
  if($checkUserFile){
   if(!(Test-Path -LiteralPath $userFile) -or (Get-Content -LiteralPath $userFile) -ne 'Keep during uninstall'){throw 'Uninstall did not preserve an unrelated file'}
   Remove-Item -LiteralPath $userFile
  }
 }
}
