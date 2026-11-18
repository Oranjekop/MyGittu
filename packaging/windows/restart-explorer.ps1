$ErrorActionPreference = 'Stop'

# GetShellWindow identifies the desktop shell, not an arbitrary folder window.
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class MyGittuDesktopShell {
    [DllImport("user32.dll")] public static extern IntPtr GetShellWindow();
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr window, out uint processId);
}
'@

function Get-ShellProcessId {
    $window = [MyGittuDesktopShell]::GetShellWindow()
    if ($window -eq [IntPtr]::Zero) { return 0 }
    [uint32]$owner = 0
    [void][MyGittuDesktopShell]::GetWindowThreadProcessId($window, [ref]$owner)
    return $owner
}

function Restart-DesktopShell {
    $session = (Get-Process -Id $PID).SessionId
    if ($session -eq 0 -or ![Environment]::UserInteractive) { return }

    $previous = Get-ShellProcessId
    $explorer = Join-Path $env:WINDIR 'explorer.exe'
    if ($previous) {
        $shell = Get-Process -Id $previous -ErrorAction SilentlyContinue
        if ($shell -and !$shell.HasExited) {
            if ($shell.SessionId -ne $session -or $shell.Path -ne $explorer) {
                throw 'The current desktop shell is not Windows Explorer.'
            }
            try {
                $shell.Kill()
            } catch {
                if (!$shell.HasExited) { throw }
            }
            if (!$shell.WaitForExit(8000)) {
                throw 'The previous Windows Explorer process did not exit.'
            }
        }
    }

    # Windows can relaunch the shell itself. A folder process is not proof
    # that the desktop taskbar is back, so wait for an actual shell window.
    $deadline = [DateTime]::UtcNow.AddSeconds(20)
    $nextLaunch = [DateTime]::UtcNow.AddSeconds(2)
    $launchError = $null
    while ([DateTime]::UtcNow -lt $deadline) {
        $current = Get-ShellProcessId
        if ($current -and $current -ne $previous) { return }
        if ([DateTime]::UtcNow -ge $nextLaunch) {
            try {
                Start-Process -FilePath $explorer
            } catch {
                $launchError = $_
            }
            $nextLaunch = [DateTime]::UtcNow.AddSeconds(5)
        }
        Start-Sleep -Milliseconds 250
    }
    if ($launchError) { throw $launchError }
    throw 'Windows Explorer did not recreate the desktop shell.'
}

if ($MyInvocation.InvocationName -ne '.') {
    Restart-DesktopShell
}
