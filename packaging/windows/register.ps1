param([switch]$Uninstall,[string]$Directory=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../bin')))
$ErrorActionPreference='Stop'
$dll=Join-Path $Directory 'MyGittuShell.dll'
$base='HKCU:\Software\Classes'
$ids=@(1, 2, 3, 4, 6, 7, 8, 9, 10) | ForEach-Object { '{29810E41-632A-4A9B-A190-4D659366810'+$_.ToString('x')+'}' }
if (!$Uninstall -and !(Test-Path -LiteralPath $dll)) { throw "Build MyGittuShell.dll into $Directory first" }
foreach ($id in $ids) {
 $key="$base\CLSID\$id"
 if ($Uninstall) { Remove-Item -LiteralPath $key -Recurse -Force -ErrorAction SilentlyContinue; continue }
 New-Item "$key\InprocServer32" -Force | Out-Null
 Set-Item "$key\InprocServer32" $dll
 New-ItemProperty "$key\InprocServer32" -Name ThreadingModel -Value Apartment -Force | Out-Null
}
foreach ($type in @('*','Directory','Directory\Background')) {
 $key="$base\$type\shellex\ContextMenuHandlers\MyGittu"
 if ($Uninstall) { Remove-Item -LiteralPath $key -Force -ErrorAction SilentlyContinue } else { New-Item $key -Force | Out-Null; Set-Item $key $ids[0] }
}
# Per-user COM/menu registration works without elevation. Overlay registration
# lives under HKLM and must be run from an elevated shell when desired.
$overlay='HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\ShellIconOverlayIdentifiers'
$admin=([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if ($admin) {
 $names=@('MyGittu Clean','MyGittu Modified','MyGittu Conflict','MyGittu State Added','MyGittu State Deleted','MyGittu State Untracked','MyGittu State Ignored','MyGittu State Upstream')
 for ($i=0;$i -lt $names.Count;$i++) {
  $legacyKey="$overlay\$($names[$i])"
  $key="$overlay\ $($names[$i])"
  if ($Uninstall) { Remove-Item -LiteralPath $key -Force -ErrorAction SilentlyContinue }
  else { New-Item -Path $key -Force | Out-Null; Set-Item -LiteralPath $key -Value $ids[$i+1] }
  Remove-Item -LiteralPath $legacyKey -Force -ErrorAction SilentlyContinue
 }
} else { Write-Host 'Menus updated. Run elevated to register/remove optional overlays.' }
Write-Host 'Restart Explorer to reload shell extensions. Windows may limit available overlay slots.'
