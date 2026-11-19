param([string]$Version=(Get-Content "$PSScriptRoot/../../VERSION" -Raw).Trim())
$ErrorActionPreference='Stop'
if($Version -notmatch '^\d+\.\d+\.\d+$'){throw 'Invalid release version'}
Push-Location "$PSScriptRoot/../.."
$previousArch=$env:GOARCH
try {
 foreach($arch in @('amd64','arm64')) {
  $folder=if($arch -eq 'amd64'){'x64'}else{'arm64'}
  $native=if($arch -eq 'amd64'){'x64'}else{'ARM64'}
  $env:GOARCH=$arch
  New-Item -ItemType Directory -Force "dist/windows/$folder" | Out-Null
  $flags="-s -w -X mygittu/internal/buildinfo.Version=$Version"
  go build -trimpath -ldflags "-H=windowsgui $flags" -o "dist/windows/$folder/agent.exe" ./cmd/agent
  if($LASTEXITCODE){throw "Agent $arch build failed"}
  go build -trimpath -ldflags "-H=windowsgui $flags" -o "dist/windows/$folder/desktop.exe" ./cmd/desktop
  if($LASTEXITCODE){throw "Desktop $arch build failed"}
  go build -trimpath -ldflags $flags -o "dist/windows/$folder/mygittu.exe" ./cmd/mygittu
  if($LASTEXITCODE){throw "CLI $arch build failed"}
  cmake -S extensions/windows -B ".work/release-$arch" -A $native
  if($LASTEXITCODE){throw "CMake $arch configure failed"}
  cmake --build ".work/release-$arch" --config Release
  if($LASTEXITCODE){throw "Shell $arch build failed"}
  Copy-Item ".work/release-$arch/Release/MyGittuShell.dll" "dist/windows/$folder/"
 }
 $iscc=(Get-Command ISCC.exe -ErrorAction SilentlyContinue).Source
 if(!$iscc){$iscc=Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6/ISCC.exe'}
 $setupArgs=@("/DAppVersion=$Version")
 if($env:MYGITTU_CERT_PATH){
  & "$PSScriptRoot/build-sparse.ps1" -Version $Version -CertificatePath $env:MYGITTU_CERT_PATH
  $setupArgs+= "/DModernShellPackage=dist\release\MyGittu-$Version-Shell.msix"
  $password = ConvertTo-SecureString $env:MYGITTU_CERT_PASSWORD -AsPlainText -Force
  $cert = (Get-PfxData -FilePath $env:MYGITTU_CERT_PATH -Password $password).EndEntityCertificates[0]
  if ($cert.Subject -eq $cert.Issuer) {
   Export-Certificate -Cert $cert -FilePath dist/release/MyGittu-Windows-Publisher.cer | Out-Null
   $setupArgs+= '/DModernShellCertificate=dist\release\MyGittu-Windows-Publisher.cer'
   $setupArgs+= "/DModernShellThumbprint=$($cert.Thumbprint)"
  }
 }
 & $iscc @setupArgs packaging/windows/installer.iss
 if($LASTEXITCODE){throw 'Installer compilation failed'}
}finally{$env:GOARCH=$previousArch;Pop-Location}
