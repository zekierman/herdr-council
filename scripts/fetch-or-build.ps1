# Install step for Windows: download the prebuilt Council binary for this plugin version and verify
# its checksum; if that fails for any reason, build from source with Go.
$ErrorActionPreference = 'Stop'
$Repo = 'zekierman/herdr-council'

function Strip-Verbatim([string]$Path) {
    # herdr may hand paths over in \\?\ form; .NET and Join-Path don't all accept it.
    if ($Path -and $Path.StartsWith('\\?\')) { return $Path.Substring(4) }
    return $Path
}

$Root = [IO.Path]::GetFullPath((Join-Path (Strip-Verbatim $PSScriptRoot) '..'))
$Manifest = Get-Content -LiteralPath (Join-Path $Root 'herdr-plugin.toml') -Raw
$Version = [regex]::Match($Manifest, '(?m)^version\s*=\s*"([^"]+)"').Groups[1].Value
$BinDir = Join-Path $Root 'bin'
$Out = Join-Path $BinDir 'council.exe'
$HookOut = Join-Path $BinDir 'council-hook.exe'   # GUI-subsystem build for herdr hooks: no console flash
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

function Build-FromSource([string]$Reason) {
    [Console]::Error.WriteLine("herdr-council: $Reason; building from source instead.")
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        [Console]::Error.WriteLine("herdr-council needs a prebuilt release for v$Version or Go from https://go.dev/dl")
        exit 1
    }
    Push-Location $Root
    try {
        $env:CGO_ENABLED = '0'
        & go build -trimpath -o $Out .
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        & go build -trimpath -ldflags '-H=windowsgui' -o $HookOut .
        exit $LASTEXITCODE
    } finally { Pop-Location }
}

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) { 'AMD64' { 'amd64' } 'ARM64' { 'arm64' } default { '' } }
if (-not $Arch) { Build-FromSource "no prebuilt binary for $env:PROCESSOR_ARCHITECTURE" }
$Asset = "council-windows-$Arch.exe"
$HookAsset = "council-windows-$Arch-hook.exe"
$Base = "https://github.com/$Repo/releases/download/v$Version"
$Tmp = Join-Path ([IO.Path]::GetTempPath()) ("herdr-council-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $Tmp | Out-Null
try {
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$Base/$Asset" -OutFile (Join-Path $Tmp $Asset)
        Invoke-WebRequest -UseBasicParsing -Uri "$Base/$HookAsset" -OutFile (Join-Path $Tmp $HookAsset)
        Invoke-WebRequest -UseBasicParsing -Uri "$Base/checksums.txt" -OutFile (Join-Path $Tmp 'checksums.txt')
    } catch { Build-FromSource "could not download $Asset for v$Version" }
    $Sums = Get-Content -LiteralPath (Join-Path $Tmp 'checksums.txt')
    foreach ($Name in @($Asset, $HookAsset)) {
        $Line = $Sums | Where-Object { $_ -match " $([regex]::Escape($Name))$" } | Select-Object -First 1
        $Want = if ($Line) { ($Line -split '\s+')[0].ToLower() } else { '' }
        $Got = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $Tmp $Name)).Hash.ToLower()
        if (-not $Want -or $Want -ne $Got) { Build-FromSource "checksum mismatch for $Name" }
    }
    Move-Item -Force -LiteralPath (Join-Path $Tmp $Asset) -Destination $Out
    Move-Item -Force -LiteralPath (Join-Path $Tmp $HookAsset) -Destination $HookOut
    Write-Output "herdr-council: installed v$Version (windows/$Arch)"
} finally {
    Remove-Item -LiteralPath $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}
