param([Parameter(Mandatory=$true)][string]$InstallPath)

$ErrorActionPreference = 'Stop'
$target = [IO.Path]::Combine($InstallPath, 'MyGittuShell.dll')
$legacy = "$target.new"
$incoming = "$target.stage"

function Clear-LegacyShellMoves($SessionManager, [string]$Target, [string]$Legacy) {
    # Session Manager stores source/destination pairs; an empty destination means delete.
    $prefix = '\??\'
    foreach ($name in 'PendingFileRenameOperations', 'PendingFileRenameOperations2') {
        $moves = $SessionManager.GetValue($name)
        if (!$moves -or $moves.Count % 2 -ne 0) { continue }
        $keep = [Collections.Generic.List[string]]::new()
        for ($i = 0; $i -lt $moves.Count; $i += 2) {
            $source = $moves[$i]
            $destination = $moves[$i + 1]
            if ((!$destination -and $source -in ("$prefix$Target", "$prefix$Legacy")) -or
                ($source -eq "$prefix$Legacy" -and $destination.TrimStart('!') -eq "$prefix$Target")) {
                continue
            }
            $keep.Add($source)
            $keep.Add($destination)
        }
        if ($keep.Count -eq $moves.Count) { continue }
        if ($keep.Count) {
            $SessionManager.SetValue($name, $keep.ToArray(), [Microsoft.Win32.RegistryValueKind]::MultiString)
        } else {
            $SessionManager.DeleteValue($name, $false)
        }
    }
}

function Get-ShellHash([string]$Path) {
    # The elevated installer can inherit a PSModulePath without Windows PowerShell modules.
    $share = [IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete
    $file = [IO.File]::Open($Path, [IO.FileMode]::Open, [IO.FileAccess]::Read, $share)
    try {
        $sha = [Security.Cryptography.SHA256]::Create()
        try {
            [Convert]::ToBase64String($sha.ComputeHash($file))
        } finally {
            $sha.Dispose()
        }
    } finally {
        $file.Dispose()
    }
}

$registry = [Microsoft.Win32.RegistryKey]::OpenBaseKey('LocalMachine', 'Registry64')
try {
    $sessionManager = $registry.OpenSubKey('SYSTEM\CurrentControlSet\Control\Session Manager', $true)
    try {
        Clear-LegacyShellMoves $sessionManager ([IO.Path]::GetFullPath($target)) ([IO.Path]::GetFullPath($legacy))
    } finally {
        $sessionManager.Dispose()
    }
} finally {
    $registry.Dispose()
}

if ([IO.File]::Exists($target)) {
    if ((Get-ShellHash $target) -ceq (Get-ShellHash $incoming)) {
        [IO.File]::Delete($incoming)
    } else {
        # A loaded DLL can be renamed even though Windows cannot delete its image.
        $backup = "$target.retired-$([guid]::NewGuid().ToString('N'))"
        [IO.File]::Move($target, $backup)
        try {
            [IO.File]::Move($incoming, $target)
        } catch {
            [IO.File]::Move($backup, $target)
            throw
        }
    }
} else {
    [IO.File]::Move($incoming, $target)
}

$retiredFiles = [IO.Directory]::EnumerateFiles($InstallPath, 'MyGittuShell.dll.retired-*')
foreach ($retired in @($legacy) + @($retiredFiles)) {
    if ($retired -ne $legacy -and [IO.Path]::GetFileName($retired) -notmatch '^MyGittuShell\.dll\.retired-[a-f0-9]{32}$') {
        continue
    }
    try {
        [IO.File]::Delete($retired)
    } catch [IO.IOException], [UnauthorizedAccessException] {
        # Explorer can still have the retired image loaded. Retry on a later install.
    }
}
