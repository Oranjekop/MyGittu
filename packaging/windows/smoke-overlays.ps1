$overlaySmokeRoot = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\ShellIconOverlayIdentifiers'
$overlaySmokeNames = @('MyGittu Clean','MyGittu Modified','MyGittu Conflict')

function Initialize-OverlaySmoke($Root) {
    & (Join-Path $Root 'integration.ps1') -Action Overlays -State On -Elevated
    Test-OverlayMigration
}

function Test-OverlayMigration {
    for ($i=0; $i -lt $overlaySmokeNames.Count; $i++) {
        $name = $overlaySmokeNames[$i]
        $key = Get-Item -LiteralPath (Join-Path $overlaySmokeRoot " $name")
        if ($key.GetValue('') -ne ('{29810E41-632A-4A9B-A190-4D659366810' + ($i+2) + '}') -or
            (Test-Path -LiteralPath (Join-Path $overlaySmokeRoot $name))) {
            throw "Overlay key did not migrate: $name"
        }
    }
}

function Test-InstalledOverlays($Root,$ShellSmoke) {
    $fixture = Join-Path ([IO.Path]::GetTempPath()) ('mygittu-overlay-' + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $fixture | Out-Null
    function Invoke-FixtureGit {
        & git -C $fixture @args
        if ($LASTEXITCODE) { throw "Fixture git failed: $args" }
    }
    try {
        Invoke-FixtureGit init -b main -q
        Invoke-FixtureGit config user.name 'MyGittu Smoke'
        Invoke-FixtureGit config user.email 'smoke@example.invalid'
        $file = Join-Path $fixture 'sample.txt'
        Set-Content -LiteralPath $file 'base'
        Invoke-FixtureGit add sample.txt
        Invoke-FixtureGit commit -qm base
        Invoke-FixtureGit branch alternate
        $slots = @()
        foreach ($expected in 1..3) {
            if ($expected -eq 2) { Set-Content -LiteralPath $file 'main' }
            if ($expected -eq 3) {
                Invoke-FixtureGit commit -qam main
                Invoke-FixtureGit switch -q alternate
                Set-Content -LiteralPath $file 'alternate'
                Invoke-FixtureGit commit -qam alternate
                Invoke-FixtureGit switch -q main
                & git -C $fixture merge alternate
                if ($LASTEXITCODE -ne 1) { throw 'Expected a fixture merge conflict' }
            }
            $timer = [Diagnostics.Stopwatch]::StartNew()
            do {
                $output = & $ShellSmoke (Join-Path $Root 'MyGittuShell.dll') $fixture $expected --explorer
                if (!$LASTEXITCODE) { break }
                Start-Sleep -Milliseconds 250
            } while ($timer.Elapsed.TotalSeconds -lt 12)
            if ($LASTEXITCODE) { throw "Installed overlay did not reach state ${expected}: $output" }
            $slot = $output | Where-Object { $_ -match '^shell overlay=\d+$' } | Select-Object -First 1
            if (!$slot -or $slot -in $slots) { throw "Missing or repeated Shell slot for state ${expected}: $output" }
            $slots += $slot
            Write-Host "Installed overlay state $expected matched at $slot after $($timer.ElapsedMilliseconds) ms"
            if ($expected -eq 2) {
                & $ShellSmoke (Join-Path $Root 'MyGittuShell.dll') $file 2
                if ($LASTEXITCODE) { throw "File context menu smoke test failed: $LASTEXITCODE" }
            }
        }
        & git -C $fixture merge --abort
        if ($LASTEXITCODE) { throw 'Could not restore the overlay fixture' }
        $tests = @(
            @{ Name = 'added.txt'; State = 4 },
            @{ Name = 'sample.txt'; State = 5 },
            @{ Name = 'untracked.txt'; State = 6 },
            @{ Name = 'ignored.txt'; State = 7 }
        )
        Set-Content -LiteralPath (Join-Path $fixture '.git/info/exclude') 'ignored.txt'
        foreach ($name in 'added.txt', 'untracked.txt', 'ignored.txt') {
            Set-Content -LiteralPath (Join-Path $fixture $name) $name
        }
        Invoke-FixtureGit add added.txt
        Invoke-FixtureGit rm sample.txt
        foreach ($test in $tests) {
            $path = Join-Path $fixture $test.Name
            $timer = [Diagnostics.Stopwatch]::StartNew()
            do {
                $output = & $ShellSmoke (Join-Path $Root 'MyGittuShell.dll') $path $test.State --overlay-only
                if (!$LASTEXITCODE) { break }
                Start-Sleep -Milliseconds 250
            } while ($timer.Elapsed.TotalSeconds -lt 12)
            if ($LASTEXITCODE) { throw "Overlay for $($test.Name) did not reach state $($test.State): $output" }
        }
    } finally {
        $resolved = (Resolve-Path -LiteralPath $fixture).Path
        $temp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
        if ([IO.Path]::GetDirectoryName($resolved) -ne $temp -or !(Split-Path $resolved -Leaf).StartsWith('mygittu-overlay-')) {
            throw "Unexpected fixture path: $resolved"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}

function Set-LegacyOverlaySmokeRegistration {
    foreach ($name in $overlaySmokeNames) {
        $current = Join-Path $overlaySmokeRoot " $name"
        $source = Get-Item -LiteralPath $current
        $legacy = New-Item -Path (Join-Path $overlaySmokeRoot $name) -Force
        try { $legacy.SetValue('', $source.GetValue('')) } finally { $legacy.Dispose(); $source.Dispose() }
        Remove-Item -LiteralPath $current
    }
}
