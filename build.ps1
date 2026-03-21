# build.ps1 — 跨平台构建打包脚本 (PowerShell) | Cross-platform build & package script
# 用法 Usage: .\build.ps1 [-Target all|windows|linux|darwin]
param(
    [string]$Target = "all"
)

$ErrorActionPreference = "Stop"

$AppName   = "go-port-forward"
$Version   = if ($env:VERSION) { $env:VERSION } else {
    $v = git describe --tags --always --dirty 2>$null
    if ($v) { $v } else { "dev" }
}
$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$OutputDir = "dist"

$LDFlags = "-s -w -X main.version=$Version -X main.buildTime=$BuildTime"

# 目标平台列表 | Target platforms
$Platforms = @(
    @{ OS = "windows"; Arch = "amd64" },
    @{ OS = "windows"; Arch = "arm64" },
    @{ OS = "linux";   Arch = "amd64" },
    @{ OS = "linux";   Arch = "arm64" },
    @{ OS = "linux";   Arch = "arm"   },
    @{ OS = "darwin";  Arch = "amd64" },
    @{ OS = "darwin";  Arch = "arm64" }
)

function Log($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Err($msg) { Write-Host "==> $msg" -ForegroundColor Red }

function Build-One {
    param([string]$OS, [string]$Arch)

    $ext = if ($OS -eq "windows") { ".exe" } else { "" }
    $binName = "${AppName}${ext}"
    $dirName = "${AppName}-${Version}-${OS}-${Arch}"
    $outDir  = Join-Path $OutputDir $dirName

    New-Item -ItemType Directory -Path $outDir -Force | Out-Null

    Log "Building ${OS}/${Arch} ..."
    $env:CGO_ENABLED = "0"
    $env:GOOS   = $OS
    $env:GOARCH = $Arch
    go build -trimpath -ldflags $LDFlags -o (Join-Path $outDir $binName) .
    if ($LASTEXITCODE -ne 0) { throw "Build failed for ${OS}/${Arch}" }

    # 复制配置示例 | Copy sample config
    if (Test-Path "config.yaml") {
        Copy-Item "config.yaml" (Join-Path $outDir "config.yaml.example")
    }

    # 打包 | Package
    $archiveName = if ($OS -eq "windows") { "${dirName}.zip" } else { "${dirName}.tar.gz" }
    $archivePath = Join-Path $OutputDir $archiveName

    if ($OS -eq "windows") {
        Compress-Archive -Path $outDir -DestinationPath $archivePath -Force
    } else {
        tar -czf $archivePath -C $OutputDir $dirName
    }

    Log "Packaged: $archivePath"
}

function Build-Targets {
    param([string]$Filter)

    foreach ($p in $Platforms) {
        if ($Filter -eq "all" -or $Filter -eq $p.OS) {
            Build-One -OS $p.OS -Arch $p.Arch
        }
    }
}

function Generate-Checksums {
    Log "Generating checksums ..."
    $files = Get-ChildItem -Path $OutputDir -Include "*.tar.gz","*.zip" -File
    $checksums = @()
    foreach ($f in $files) {
        $hash = (Get-FileHash -Path $f.FullName -Algorithm SHA256).Hash.ToLower()
        $checksums += "$hash  $($f.Name)"
    }
    $checksums | Set-Content (Join-Path $OutputDir "checksums-sha256.txt") -Encoding UTF8
    Log "Checksums: $OutputDir/checksums-sha256.txt"
}

# ── Main ──
Log "${AppName} build script (PowerShell)"
Log "Version: $Version"
Log "Target:  $Target"

# Clean
if (Test-Path $OutputDir) {
    Log "Cleaning $OutputDir/ ..."
    Remove-Item -Recurse -Force $OutputDir
}
New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null

Build-Targets -Filter $Target
Generate-Checksums

Log "Done! All artifacts in $OutputDir/"
Get-ChildItem -Path $OutputDir -Include "*.tar.gz","*.zip" -File | Format-Table Name, @{N="Size";E={"{0:N1} MB" -f ($_.Length / 1MB)}} -AutoSize

