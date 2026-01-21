# ==============================
# Go 全平台交叉编译脚本（Windows）
# ==============================

$projectName = "ratel-server"
$mainFile    = "main.go"
$targetDir   = "target"

# 创建输出目录
if (!(Test-Path $targetDir)) {
    New-Item -ItemType Directory -Path $targetDir | Out-Null
    Write-Host "已创建目录: $targetDir" -ForegroundColor DarkGray
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

Write-Host "`n开始 Go 全平台编译：$projectName`n" -ForegroundColor Cyan

foreach ($p in $platforms) {
    $goos   = $p.os
    $goarch = $p.arch

    $ext = if ($goos -eq "windows") { ".exe" } else { "" }

    $outputName = "$projectName-$goos-$goarch$ext"
    $outputPath = Join-Path $targetDir $outputName

    Write-Host "▶ 编译 $goos/$goarch ..." -NoNewline

    # 关键环境变量
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    $env:CGO_ENABLED = "0"

    go build -trimpath -ldflags "-s -w" `
        -o $outputPath $mainFile 2>$null

    if ($LASTEXITCODE -eq 0) {
        Write-Host " 成功 -> $outputName" -ForegroundColor Green
    } else {
        Write-Host " 失败" -ForegroundColor Red
    }
}

# 清理环境变量
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

Write-Host "`n所有平台编译完成，输出目录：$targetDir" -ForegroundColor Yellow
