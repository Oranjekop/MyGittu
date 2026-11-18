param([Parameter(Mandatory=$true)][string]$InstallPath)

$ErrorActionPreference = 'Stop'
$names = @('desktop', 'mygittu', 'agent')
$paths = @($names | ForEach-Object {
    [IO.Path]::GetFullPath((Join-Path $InstallPath "$_.exe"))
})
foreach ($name in $names) {
    $running = @(Get-Process -Name $name -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -in $paths })
    foreach ($process in $running) {
        try {
            if ($process.HasExited) { continue }
            if ($process.CloseMainWindow() -and $process.WaitForExit(2000)) { continue }
            if (!$process.HasExited) { $process.Kill() }
        } catch {
            if (!$process.HasExited) { throw }
        }
        if (!$process.WaitForExit(5000)) {
            throw "MyGittu $name.exe did not stop."
        }
    }
}
