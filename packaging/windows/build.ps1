$ErrorActionPreference='Stop'
Push-Location "$PSScriptRoot/../.."
try {
 go build -o bin/agent.exe ./cmd/agent
 if ($LASTEXITCODE) { throw 'Agent build failed' }
 go build -ldflags=-H=windowsgui -o bin/desktop.exe ./cmd/desktop
 if ($LASTEXITCODE) { throw 'Desktop build failed' }
 go build -o bin/mygittu.exe ./cmd/mygittu
 if ($LASTEXITCODE) { throw 'CLI build failed' }
 cmake -S extensions/windows -B .work/windows -A x64
 if ($LASTEXITCODE) { throw 'CMake configure failed: install Visual Studio C++ build tools and CMake' }
 cmake --build .work/windows --config Release
 if ($LASTEXITCODE) { throw 'Shell build failed' }
 Copy-Item -LiteralPath '.work/windows/Release/MyGittuShell.dll' -Destination 'bin/MyGittuShell.dll'
} finally { Pop-Location }
