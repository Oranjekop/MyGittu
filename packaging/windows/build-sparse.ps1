param(
 [string]$Version=(Get-Content "$PSScriptRoot/../../VERSION" -Raw).Trim(),
 [string]$Publisher='CN=Oranjekop',
 [string]$CertificatePath,
 [string]$CertificatePassword=$env:MYGITTU_CERT_PASSWORD
)
$ErrorActionPreference='Stop'
if($Version -notmatch '^\d+\.\d+\.\d+$'){throw 'Invalid version'}
$root=(Resolve-Path "$PSScriptRoot/../..").Path
$sdk=Get-ChildItem "${env:ProgramFiles(x86)}/Windows Kits/10/bin/*/x64/makeappx.exe" | Sort-Object FullName -Descending | Select-Object -First 1
if(!$sdk){throw 'Windows SDK makeappx.exe is required'}
$stage=Join-Path $root '.work/sparse-package'
New-Item -ItemType Directory -Force "$stage/Assets","$root/dist/release" | Out-Null
[xml]$manifest=Get-Content "$PSScriptRoot/sparse/AppxManifest.xml"
$manifest.Package.Identity.Version="$Version.0"
$manifest.Package.Identity.Publisher=$Publisher
$manifest.Save("$stage/AppxManifest.xml")
Add-Type -AssemblyName System.Drawing
$image=[Drawing.Image]::FromFile("$root/internal/brand/icon-ui.png")
try {
 foreach($asset in @(@('StoreLogo',50),@('Logo44',44),@('Logo150',150))){
  $bitmap=[Drawing.Bitmap]::new([int]$asset[1],[int]$asset[1])
  $graphics=[Drawing.Graphics]::FromImage($bitmap)
  try{$graphics.InterpolationMode=[Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic;$graphics.DrawImage($image,0,0,$bitmap.Width,$bitmap.Height);$bitmap.Save("$stage/Assets/$($asset[0]).png",[Drawing.Imaging.ImageFormat]::Png)}finally{$graphics.Dispose();$bitmap.Dispose()}
 }
}finally{$image.Dispose()}
$package="$root/dist/release/MyGittu-$Version-Shell.msix"
& $sdk.FullName pack /d $stage /p $package /nv /o
if($LASTEXITCODE){throw 'Sparse package build failed'}
if($CertificatePath){
 $signTool=Join-Path $sdk.DirectoryName 'signtool.exe'
 # Password stays out of repository configuration; CI masks the secret value.
 & $signTool sign /fd SHA256 /f $CertificatePath /p $CertificatePassword /tr http://timestamp.digicert.com /td SHA256 $package
 if($LASTEXITCODE){throw 'Sparse package signing failed'}
 & $signTool verify /pa $package
 if($LASTEXITCODE){throw 'Sparse package signature verification failed'}
}else{Write-Host 'Built unsigned sparse package. A trusted publisher signature is required before installation.'}
