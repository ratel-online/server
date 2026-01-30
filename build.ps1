# ==============================
# Go Multi-platform Build Script
# ==============================

$projectName = "ratel-server"
$mainFile    = "main.go"
$targetDir   = "target"

# Create output directory
if (!(Test-Path $targetDir)) {
    New-Item -ItemType Directory -Path $targetDir | Out-Null
    Write-Host "Created directory: $targetDir" -ForegroundColor DarkGray
}

# 目标平台列表
$platforms = @(
    @{ os = "windows"; arch = "amd64" },
    @{ os = "windows"; arch = "386"   },
    @{ os = "windows"; arch = "arm64" },

    @{ os = "linux";   arch = "amd64" },
    @{ os = "linux";   arch = "386"   },
    @{ os = "linux";   arch = "arm64" },
    @{ os = "linux";   arch = "arm"   },

    @{ os = "darwin";  arch = "amd64" },
    @{ os = "darwin";  arch = "arm64" }
)

Write-Host "`nStarting Go multi-platform build: $projectName`n" -ForegroundColor Cyan

foreach ($p in $platforms) {
    $goos   = $p.os
    $goarch = $p.arch

    $ext = if ($goos -eq "windows") { ".exe" } else { "" }

    $outputName = "$projectName-$goos-$goarch$ext"
    $outputPath = Join-Path $targetDir $outputName

    Write-Host "> Building $goos/$goarch ..." -NoNewline

    # Environment variables
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    $env:CGO_ENABLED = "0"

    go build -trimpath -ldflags "-s -w" `
        -o $outputPath $mainFile 2>$null

    if ($LASTEXITCODE -eq 0) {
        Write-Host " Success -> $outputName" -ForegroundColor Green
    } else {
        Write-Host " Failed" -ForegroundColor Red
    }
}

# 清理环境变量
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

Write-Host "`nAll platforms build completed. Output directory: $targetDir" -ForegroundColor Yellow
