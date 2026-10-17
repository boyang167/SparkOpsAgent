# SparkOps 本地生产模式：编译并启动前后端（用于性能对比，非日常开发）
# 用法:
#   .\scripts\dev-restart-prod.ps1              # 编译并重启后端 :8080 + 前端 :3000
#   .\scripts\dev-restart-prod.ps1 -Stop        # 仅停止
#   .\scripts\dev-restart-prod.ps1 -SkipBuild   # 跳过编译，直接启动（代码未改时用）
#   .\scripts\dev-restart-prod.ps1 -Backend     # 只重启后端
#   .\scripts\dev-restart-prod.ps1 -Frontend    # 只重启前端
#
# 日常开发请用: .\scripts\dev-restart.ps1

param(
    [switch]$Stop,
    [switch]$SkipBuild,
    [switch]$Backend,
    [switch]$Frontend
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$BackendDir = Join-Path $RepoRoot "backend"
$FrontedDir = Join-Path $RepoRoot "fronted"
$DevDir = Join-Path $RepoRoot ".dev"
$PidFile = Join-Path $DevDir "dev-prod.pids.json"
$BackendBinary = Join-Path $DevDir "sparkops-server.exe"

$BackendPort = 8080
$FrontendPort = 3000

function Ensure-DevDir {
    if (-not (Test-Path $DevDir)) {
        New-Item -ItemType Directory -Path $DevDir | Out-Null
    }
}

function Stop-PortListener {
    param([int]$Port)

    $found = $false
    try {
        $connections = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        foreach ($conn in $connections) {
            $processId = $conn.OwningProcess
            if ($processId -and $processId -ne 0) {
                $found = $true
                Write-Host "  停止端口 $Port 上的进程 PID=$processId" -ForegroundColor Yellow
                Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
            }
        }
    } catch {
        # 部分 Windows 环境无 Get-NetTCPConnection，回退 netstat
    }

    if (-not $found) {
        $pattern = ":\s*$Port\s"
        netstat -ano | Select-String $pattern | ForEach-Object {
            $parts = ($_.Line -split '\s+') | Where-Object { $_ -ne '' }
            if ($parts.Length -ge 5 -and $parts[3] -eq "LISTENING") {
                $processId = [int]$parts[4]
                if ($processId -gt 0) {
                    Write-Host "  停止端口 $Port 上的进程 PID=$processId" -ForegroundColor Yellow
                    Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
                }
            }
        }
    }
}

function Load-Pids {
    if (-not (Test-Path $PidFile)) { return @{} }
    try {
        $saved = Get-Content $PidFile -Raw | ConvertFrom-Json
        $obj = @{}
        foreach ($name in @("backend", "frontend")) {
            $entry = $saved.$name
            if ($null -eq $entry) { continue }
            $obj[$name] = @{
                processId = [int]$entry.processId
                port      = [int]$entry.port
                url       = [string]$entry.url
                startedAt = [string]$entry.startedAt
            }
        }
        return $obj
    } catch {
        Write-Host "  忽略无效 PID 文件: $PidFile" -ForegroundColor DarkGray
        return @{}
    }
}

function Stop-SavedPids {
    param(
        [bool]$IncludeBackend,
        [bool]$IncludeFrontend
    )

    $saved = Load-Pids
    if ($saved.Count -eq 0) { return }

    foreach ($name in @("backend", "frontend")) {
        if ($name -eq "backend" -and -not $IncludeBackend) { continue }
        if ($name -eq "frontend" -and -not $IncludeFrontend) { continue }
        $entry = $saved[$name]
        if ($null -eq $entry) { continue }
        $processId = [int]$entry.processId
        if ($processId -gt 0 -and (Get-Process -Id $processId -ErrorAction SilentlyContinue)) {
            Write-Host "  停止 $name (PID=$processId)" -ForegroundColor Yellow
            Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
        }
    }
}

function Stop-ProdServices {
    param(
        [bool]$IncludeBackend,
        [bool]$IncludeFrontend
    )

    Write-Host ">> 停止生产模式服务..." -ForegroundColor Cyan
    Stop-SavedPids -IncludeBackend $IncludeBackend -IncludeFrontend $IncludeFrontend
    if ($IncludeBackend) { Stop-PortListener -Port $BackendPort }
    if ($IncludeFrontend) { Stop-PortListener -Port $FrontendPort }
    Start-Sleep -Milliseconds 500
}

function Test-PortListening {
    param([int]$Port)

    try {
        $connections = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        if ($connections) { return $true }
    } catch {
        # fall through to netstat
    }

    $pattern = ":\s*$Port\s"
    return [bool](netstat -ano | Select-String $pattern | Select-String "LISTENING")
}

function Wait-BackendReady {
    param(
        [int]$Port,
        [int]$TimeoutSec = 30
    )

    Write-Host "  等待后端监听 :$Port ..." -ForegroundColor DarkGray
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-PortListening -Port $Port) {
            try {
                $resp = Invoke-WebRequest -Uri "http://localhost:$Port/api/v1/auth/providers" -UseBasicParsing -TimeoutSec 3
                if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
                    return $true
                }
            } catch {
                $status = $null
                if ($_.Exception.Response) {
                    $status = [int]$_.Exception.Response.StatusCode
                }
                if ($status -ge 400 -and $status -lt 500) {
                    return $true
                }
            }
        }
        Start-Sleep -Seconds 1
    }
    return $false
}

function Wait-FrontendReady {
    param(
        [int]$Port,
        [int]$TimeoutSec = 30
    )

    Write-Host "  等待前端监听 :$Port ..." -ForegroundColor DarkGray
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-PortListening -Port $Port) {
            return $true
        }
        Start-Sleep -Seconds 1
    }
    return $false
}

function Resolve-PnpmCmd {
    $cmd = Get-Command pnpm.cmd -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }

    $npmDir = Join-Path $env:APPDATA "npm"
    $fallback = Join-Path $npmDir "pnpm.cmd"
    if (Test-Path $fallback) { return $fallback }

    throw "未找到 pnpm.cmd，请先安装: npm install -g pnpm"
}

function Build-Backend {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "未找到 go 命令，请先安装 Go 并加入 PATH"
    }
    if (-not (Test-Path (Join-Path $BackendDir "cmd\server"))) {
        throw "未找到 backend/cmd/server，请确认仓库路径: $BackendDir"
    }

    Ensure-DevDir
    Write-Host ">> 编译后端（生产二进制）..." -ForegroundColor Green
    Push-Location $BackendDir
    try {
        & go build -o $BackendBinary ./cmd/server
        if ($LASTEXITCODE -ne 0) {
            throw "后端编译失败，退出码 $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

function Build-Frontend {
    if (-not (Test-Path (Join-Path $FrontedDir "package.json"))) {
        throw "未找到 fronted/package.json，请确认仓库路径: $FrontedDir"
    }
    $pnpmCmd = Resolve-PnpmCmd

    Write-Host ">> 构建前端 (next build, 可能需要数分钟)..." -ForegroundColor Green
    $env:NODE_ENV = "production"
    Push-Location $FrontedDir
    try {
        & $pnpmCmd --filter web build
        if ($LASTEXITCODE -ne 0) {
            throw "前端构建失败，退出码 $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

function Start-BackendService {
    if (-not (Test-Path $BackendBinary)) {
        throw "未找到后端二进制 $BackendBinary，请先去掉 -SkipBuild 重新编译"
    }
    if (-not (Test-Path (Join-Path $BackendDir "config.yaml"))) {
        Write-Host "  提示: backend/config.yaml 不存在，可参考 config.example.yaml 创建" -ForegroundColor DarkYellow
    }

    Write-Host ">> 启动后端 http://localhost:$BackendPort" -ForegroundColor Green
    $proc = Start-Process -FilePath $BackendBinary `
        -WorkingDirectory $BackendDir `
        -PassThru `
        -WindowStyle Normal

    if (-not (Wait-BackendReady -Port $BackendPort)) {
        throw "后端在 30 秒内未就绪，请查看后端窗口中的启动错误"
    }

    return @{
        name      = "backend"
        processId = $proc.Id
        port      = $BackendPort
        url       = "http://localhost:$BackendPort"
    }
}

function Start-FrontendService {
    $pnpmCmd = Resolve-PnpmCmd
    $nextDir = Join-Path $FrontedDir "apps\web\.next"
    if (-not (Test-Path $nextDir)) {
        throw "未找到 fronted/apps/web/.next，请先去掉 -SkipBuild 重新构建"
    }

    Write-Host ">> 启动前端 http://localhost:$FrontendPort (next start)" -ForegroundColor Green
    $env:NODE_ENV = "production"
    $proc = Start-Process -FilePath $pnpmCmd `
        -ArgumentList "--filter", "web", "start" `
        -WorkingDirectory $FrontedDir `
        -PassThru `
        -WindowStyle Normal

    if (-not (Wait-FrontendReady -Port $FrontendPort)) {
        throw "前端在 30 秒内未就绪，请查看前端窗口中的启动错误"
    }

    return @{
        name      = "frontend"
        processId = $proc.Id
        port      = $FrontendPort
        url       = "http://localhost:$FrontendPort"
    }
}

function Save-Pids {
    param(
        [array]$Entries,
        [hashtable]$Existing = $null
    )

    Ensure-DevDir
    $obj = @{}
    if ($null -ne $Existing) {
        foreach ($key in $Existing.Keys) {
            $obj[$key] = $Existing[$key]
        }
    }
    foreach ($entry in $Entries) {
        $obj[$entry.name] = @{
            processId = $entry.processId
            port      = $entry.port
            url       = $entry.url
            startedAt = (Get-Date).ToString("o")
        }
    }
    $obj | ConvertTo-Json -Depth 4 | Set-Content -Path $PidFile -Encoding UTF8
}

$runBackend = (-not $Backend -and -not $Frontend) -or $Backend
$runFrontend = (-not $Backend -and -not $Frontend) -or $Frontend
$stopBackend = $Stop -or $runBackend
$stopFrontend = $Stop -or $runFrontend
$existingPids = Load-Pids

Stop-ProdServices -IncludeBackend $stopBackend -IncludeFrontend $stopFrontend

if ($Stop) {
    Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
    Write-Host ">> 已停止。" -ForegroundColor Cyan
    Write-Host "日常开发请执行: .\scripts\dev-restart.ps1" -ForegroundColor DarkGray
    exit 0
}

if (-not $SkipBuild) {
    if ($runBackend) { Build-Backend }
    if ($runFrontend) { Build-Frontend }
} else {
    Write-Host ">> 跳过编译 (-SkipBuild)" -ForegroundColor DarkGray
}

$started = @()
if ($runBackend) { $started += ,(Start-BackendService) }
if ($runFrontend) { $started += ,(Start-FrontendService) }
if ($started.Count -gt 0) {
    $retained = @{}
    if (-not $runBackend -and $existingPids.ContainsKey("backend")) {
        $retained["backend"] = $existingPids["backend"]
    }
    if (-not $runFrontend -and $existingPids.ContainsKey("frontend")) {
        $retained["frontend"] = $existingPids["frontend"]
    }
    Save-Pids -Entries $started -Existing $retained
}

Write-Host ""
Write-Host "SparkOps 生产模式服务已启动：" -ForegroundColor Cyan
foreach ($entry in $started) {
    Write-Host "  $($entry.name): $($entry.url) (PID $($entry.processId))" -ForegroundColor White
}
Write-Host ""
Write-Host "代码变更后需重新编译: .\scripts\dev-restart-prod.ps1" -ForegroundColor DarkGray
Write-Host "仅重启不编译:         .\scripts\dev-restart-prod.ps1 -SkipBuild" -ForegroundColor DarkGray
Write-Host "切回开发模式:         .\scripts\dev-restart.ps1" -ForegroundColor DarkGray
