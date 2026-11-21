param([Parameter(Mandatory=$true)][string]$InstallPath)

$ErrorActionPreference = 'Stop'
$target = [IO.Path]::Combine($InstallPath, 'MyGittuShell.dll')
$incoming = "$target.new"

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

foreach ($retired in [IO.Directory]::EnumerateFiles($InstallPath, 'MyGittuShell.dll.retired-*')) {
    if ([IO.Path]::GetFileName($retired) -notmatch '^MyGittuShell\.dll\.retired-[a-f0-9]{32}$') {
        continue
    }
    try {
        [IO.File]::Delete($retired)
    } catch [IO.IOException], [UnauthorizedAccessException] {
        # Explorer can still have the retired image loaded. Retry on a later install.
    }
}
