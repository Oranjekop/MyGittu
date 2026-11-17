param([Parameter(Mandatory=$true)][string]$Installer,[string]$Version=(Get-Content "$PSScriptRoot/../../VERSION" -Raw).Trim())
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../.work/install-smoke'))
$process=Start-Process -FilePath (Resolve-Path $Installer).Path -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/SP-',('/DIR="'+$root+'"')) -Wait -PassThru
if($process.ExitCode -ne 0){throw "Installer exited $($process.ExitCode)"}
$agent=$null
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
 Write-Host "Installed native $native package and verified Agent $Version"
}finally{
 if($agent -and !$agent.HasExited){Stop-Process -Id $agent.Id -Force}
 $uninstaller=Join-Path $root 'unins000.exe'
 if(Test-Path -LiteralPath $uninstaller){$removed=Start-Process -FilePath $uninstaller -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART') -Wait -PassThru;if($removed.ExitCode -ne 0){throw 'Uninstaller failed'}}
}
