param([Parameter(Mandatory=$true)][string]$InstallPath)

$ErrorActionPreference = 'Stop'
$agentPath = [IO.Path]::GetFullPath((Join-Path $InstallPath 'agent.exe'))
$agents = Get-Process -Name agent -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $agentPath }
foreach ($agent in $agents) {
    if (!$agent.HasExited) {
        $agent.Kill()
        if (!$agent.WaitForExit(5000)) {
            throw "MyGittu Agent did not stop."
        }
    }
}
