Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$RepoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$BuildDir = Join-Path $RepoRoot 'build'

$Targets = [ordered]@{
    conn       = @{ Source = 'cmd/connsvr';       Binary = 'connsvr.exe' }
    main       = @{ Source = 'cmd/mainsvr';       Binary = 'mainsvr.exe' }
    info       = @{ Source = 'cmd/infosvr';       Binary = 'infosvr.exe' }
    mysql      = @{ Source = 'cmd/mysqlsvr';      Binary = 'mysqlsvr.exe' }
    roomcenter = @{ Source = 'cmd/roomcentersvr'; Binary = 'roomcentersvr.exe' }
    web        = @{ Source = 'cmd/web_svr';       Binary = 'websvr.exe' }
}

$Aliases = @{
    connsvr       = 'conn'
    mainsvr       = 'main'
    infosvr       = 'info'
    mysqlsvr      = 'mysql'
    room          = 'roomcenter'
    roomcentersvr = 'roomcenter'
    websvr        = 'web'
    web_svr       = 'web'
}

function Show-Usage {
    @"
GoOne Windows build entrypoint (PowerShell)

Usage:
  .\build.ps1
  .\build.ps1 all
  .\build.ps1 list
  .\build.ps1 help
  .\build.ps1 <target> [target...]

Targets:
  conn        -> cmd/connsvr        -> build/connsvr.exe
  main        -> cmd/mainsvr        -> build/mainsvr.exe
  info        -> cmd/infosvr        -> build/infosvr.exe
  mysql       -> cmd/mysqlsvr       -> build/mysqlsvr.exe
  roomcenter  -> cmd/roomcentersvr  -> build/roomcentersvr.exe
  web         -> cmd/web_svr        -> build/websvr.exe

Aliases:
  connsvr, mainsvr, infosvr, mysqlsvr, roomcentersvr, room, websvr, web_svr

Notes:
  - This script mirrors the active target set in ./build.sh.
  - main.sh remains the primary repo entrypoint on Bash/WSL/Git-Bash.
"@
}

function Resolve-Target([string]$Target) {
    if ($Targets.Contains($Target)) {
        return $Target
    }
    if ($Aliases.ContainsKey($Target)) {
        return $Aliases[$Target]
    }
    throw "Unsupported build target: $Target`nRun .\build.ps1 help to see the active target list."
}

function Invoke-Build([string]$CanonicalTarget) {
    $targetInfo = $Targets[$CanonicalTarget]
    $outputPath = Join-Path $BuildDir $targetInfo.Binary
    Write-Host "==> building $($targetInfo.Binary) ($($targetInfo.Source))"
    Push-Location $RepoRoot
    try {
        & go build -o $outputPath (Join-Path '.' $targetInfo.Source)
    }
    finally {
        Pop-Location
    }
}

function Main([string[]]$InputTargets) {
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir | Out-Null
    }

    if (-not $InputTargets -or $InputTargets.Count -eq 0) {
        $requested = @($Targets.Keys)
    }
    else {
        $requested = New-Object System.Collections.Generic.List[string]
        foreach ($target in $InputTargets) {
            switch ($target) {
                'help' { Show-Usage; return }
                '-h' { Show-Usage; return }
                '--help' { Show-Usage; return }
                'list' { $Targets.Keys; return }
                'all' {
                    foreach ($canonical in $Targets.Keys) {
                        $requested.Add($canonical)
                    }
                }
                default {
                    $requested.Add((Resolve-Target $target))
                }
            }
        }
    }

    $seen = @{}
    foreach ($canonical in $requested) {
        if ($seen.ContainsKey($canonical)) {
            continue
        }
        $seen[$canonical] = $true
        Invoke-Build $canonical
    }

    Write-Host '==> build complete'
}

Main $args

