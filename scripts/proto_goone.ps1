Param(
  [string]$Module = "github.com/Iori372552686/GoOne",
  [string]$OutDir = ".",
  [string]$ProtoRoot = "api/proto",
  [switch]$UseWSL
)

$ErrorActionPreference = "Stop"

function Resolve-GoExe {
  $candidates = @()
  try {
    $cmd = Get-Command go -ErrorAction Stop
    if ($cmd.Source) { $candidates += $cmd.Source }
  }
  catch {}

  if ($env:GOROOT) {
    $candidates += (Join-Path $env:GOROOT "bin\go.exe")
  }
  $candidates += @(
    "C:\Go\bin\go.exe",
    "C:\Program Files\Go\bin\go.exe"
  )

  foreach ($p in $candidates | Select-Object -Unique) {
    if ($p -and (Test-Path $p)) { return $p }
  }
  return $null
}

function Invoke-InWSL([string]$repoRoot) {
  $wsl = Get-Command wsl -ErrorAction SilentlyContinue
  if (-not $wsl) {
    throw "wsl.exe not found; can't use WSL fallback."
  }
  # Use -e to avoid shell-escaping issues with backslashes in Windows paths.
  $wslPath = & wsl.exe -e wslpath -a $repoRoot
  if (-not $wslPath) {
    throw "failed to convert repo path to WSL path: $repoRoot"
  }
  Write-Host "[proto_goone] running in WSL: cd $wslPath && ./scripts/proto_goone.sh"
  & wsl.exe -e bash -lc "cd `"$wslPath`" && ./scripts/proto_goone.sh"
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$goExe = Resolve-GoExe
if (-not $goExe) {
  if ($UseWSL -or ($env:PROTO_GOONE_USE_WSL -eq "1")) {
    Invoke-InWSL $repoRoot
    exit 0
  }

  Write-Host "[proto_goone] ERROR: Go toolchain not found (go.exe)." -ForegroundColor Red
  Write-Host "[proto_goone] Fix: install Go for Windows, OR run with WSL fallback:" -ForegroundColor Yellow
  Write-Host "  .\scripts\proto_goone.ps1 -UseWSL" -ForegroundColor Yellow
  throw "go.exe not found"
}

if (Test-Path (Join-Path $repoRoot "common/game_proto/gen_code.bat")) {
  Write-Host "[proto_goone] step 1/2: generate common/protocol"
  Push-Location (Join-Path $repoRoot "commongame_proto")
  try {
    cmd /c gen_code.bat
    if ($LASTEXITCODE -ne 0) {
      throw "common/game_proto/gen_code.bat failed with exit code $LASTEXITCODE"
    }
  }
  finally {
    Pop-Location
  }
}

Write-Host "[proto_goone] step 2/2: generate GoOne api/gen via tools/cmd/genproto"
Push-Location $repoRoot
try {
  & $goExe run .\tools\cmd\genproto -module $Module -out $OutDir -proto_root $ProtoRoot
  if ($LASTEXITCODE -ne 0) {
    throw "tools/cmd/genproto failed with exit code $LASTEXITCODE"
  }
}
finally {
  Pop-Location
}

Write-Host "[proto_goone] done"
