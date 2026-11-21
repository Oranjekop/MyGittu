param([Parameter(Mandatory=$true)][string]$InstallPath)

$ErrorActionPreference = 'Stop'
$target = Join-Path $InstallPath 'MyGittuShell.dll'
$incoming = "$target.new"
if (Test-Path -LiteralPath $target) {
    if ((Get-FileHash -LiteralPath $target).Hash -eq (Get-FileHash -LiteralPath $incoming).Hash) {
        Remove-Item -LiteralPath $incoming
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

Get-ChildItem -LiteralPath $InstallPath -Filter 'MyGittuShell.dll.retired-*' -File |
    Where-Object { $_.Name -match '^MyGittuShell\.dll\.retired-[a-f0-9]{32}$' } |
    Remove-Item -ErrorAction SilentlyContinue
