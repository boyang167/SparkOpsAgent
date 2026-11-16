# SparkOps 本地开发：停止并重启 API、文件数据面和前端
# 用法:
#   .\scripts\dev-restart.ps1           # 重启 API :8080 + Gateway :8082 + Worker + 前端 :3000
#   .\scripts\dev-restart.ps1 -Stop     # 仅停止
#   .\scripts\dev-restart.ps1 -Backend  # 只重启 API + Gateway + Worker
#   .\scripts\dev-restart.ps1 -Frontend # 只重启前端

param(
    [switch]$Stop,
    [switch]$Backend,
    [switch]$Frontend
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$BackendDir = Join-Path $RepoRoot "backend"
$FrontedDir = Join-Path $RepoRoot "fronted"
$DevDir = Join-Path $RepoRoot ".dev"
$PidFile = Join-Path $DevDir "dev.pids.json"

$BackendPort = 8080
$GatewayPort = 8082
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
                Stop-ProcessTree -ProcessId $processId
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
                    Stop-ProcessTree -ProcessId $processId
                }
            }
        }
    }
}

function Stop-ProcessTree {
    param([int]$ProcessId)

    if ($ProcessId -le 0 -or -not (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)) {
        return
    }
    & taskkill.exe /PID $ProcessId /T /F 2>$null | Out-Null
}

function Load-Pids {
    if (-not (Test-Path $PidFile)) { return @{} }
    try {
        $saved = Get-Content $PidFile -Raw | ConvertFrom-Json
        $obj = @{}
        foreach ($name in @("backend", "gateway", "worker", "frontend")) {
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

    foreach ($name in @("backend", "gateway", "worker", "frontend")) {
        if ($name -in @("backend", "gateway", "worker") -and -not $IncludeBackend) { continue }
        if ($name -eq "frontend" -and -not $IncludeFrontend) { continue }
        $entry = $saved[$name]
        if ($null -eq $entry) { continue }
        $processId = [int]$entry.processId
        if ($processId -gt 0 -and (Get-Process -Id $processId -ErrorAction SilentlyContinue)) {
            Write-Host "  停止 $name (PID=$processId)" -ForegroundColor Yellow
            Stop-ProcessTree -ProcessId $processId
        }
    }
}

function Stop-DevServices {
    param(
        [bool]$IncludeBackend,
        [bool]$IncludeFrontend
    )

    Write-Host ">> 停止开发服务..." -ForegroundColor Cyan
    Stop-SavedPids -IncludeBackend $IncludeBackend -IncludeFrontend $IncludeFrontend
    if ($IncludeBackend) {
        Stop-PortListener -Port $BackendPort
        Stop-PortListener -Port $GatewayPort
    }
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
        [System.Diagnostics.Process]$Process,
        [string]$LogPath,
        [string]$ErrPath,
        [int]$TimeoutSec = 90
    )

    Write-Host "  等待后端编译并监听 :$Port ..." -ForegroundColor DarkGray
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if ($Process -and $Process.HasExited) {
            Write-Host "  后端进程已退出 (exit=$($Process.ExitCode))，最近日志:" -ForegroundColor Red
            Show-BackendStartupLog -LogPath $LogPath -ErrPath $ErrPath
            return $false
        }
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
        Start-Sleep -Seconds 2
    }
    Write-Host "  后端在 $TimeoutSec 秒内未就绪，最近日志:" -ForegroundColor Red
    Show-BackendStartupLog -LogPath $LogPath -ErrPath $ErrPath
    return $false
}

function Wait-GatewayReady {
    param(
        [int]$Port,
        [System.Diagnostics.Process]$Process,
        [string]$LogPath,
        [string]$ErrPath,
        [int]$TimeoutSec = 90
    )

    Write-Host "  等待文件传输网关监听 :$Port ..." -ForegroundColor DarkGray
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if ($Process -and $Process.HasExited) {
            Write-Host "  文件传输网关进程已退出 (exit=$($Process.ExitCode))，最近日志:" -ForegroundColor Red
            Show-BackendStartupLog -LogPath $LogPath -ErrPath $ErrPath
            return $false
        }
        if (Test-PortListening -Port $Port) {
            return $true
        }
        Start-Sleep -Seconds 2
    }
    Write-Host "  文件传输网关在 $TimeoutSec 秒内未就绪，最近日志:" -ForegroundColor Red
    Show-BackendStartupLog -LogPath $LogPath -ErrPath $ErrPath
    return $false
}

function Wait-WorkerReady {
    param(
        [System.Diagnostics.Process]$Process,
        [string]$LogPath,
        [string]$ErrPath,
        [int]$TimeoutSec = 90
    )

    Write-Host "  等待文件 Worker 启动 ..." -ForegroundColor DarkGray
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if ($Process.HasExited) {
            Write-Host "  文件 Worker 进程已退出 (exit=$($Process.ExitCode))，最近日志:" -ForegroundColor Red
            Show-BackendStartupLog -LogPath $LogPath -ErrPath $ErrPath
            return $false
        }
        if ((Test-Path -LiteralPath $ErrPath) -and
            (Select-String -LiteralPath $ErrPath -SimpleMatch "file-worker subscribers registered; waiting" -Quiet)) {
            return $true
        }
        Start-Sleep -Milliseconds 500
    }
    Write-Host "  文件 Worker 在 $TimeoutSec 秒内未就绪，最近日志:" -ForegroundColor Red
    Show-BackendStartupLog -LogPath $LogPath -ErrPath $ErrPath
    return $false
}

function Show-BackendStartupLog {
    param(
        [string]$LogPath,
        [string]$ErrPath
    )

    foreach ($path in @($ErrPath, $LogPath)) {
        if (-not $path -or -not (Test-Path -LiteralPath $path)) { continue }
        $lines = Get-Content -LiteralPath $path -Tail 40 -ErrorAction SilentlyContinue
        if ($lines) {
            Write-Host "  --- $path ---" -ForegroundColor DarkGray
            $lines | ForEach-Object { Write-Host "  $_" -ForegroundColor DarkYellow }
        }
    }
}

function Start-BackendService {
    if (-not (Test-Path (Join-Path $BackendDir "cmd\server"))) {
        throw "未找到 backend/cmd/server，请确认仓库路径: $BackendDir"
    }
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "未找到 go 命令，请先安装 Go 并加入 PATH"
    }
    if (-not (Test-Path (Join-Path $BackendDir "config.yaml"))) {
        Write-Host "  提示: backend/config.yaml 不存在，可参考 config.example.yaml 创建" -ForegroundColor DarkYellow
    }

    Ensure-DevDir
    $logPath = Join-Path $DevDir "backend-run.log"
    $errPath = Join-Path $DevDir "backend-run.err.log"
    Remove-Item -LiteralPath $logPath, $errPath -ErrorAction SilentlyContinue

    Write-Host ">> 启动后端 http://localhost:$BackendPort" -ForegroundColor Green
    Write-Host "  启动日志: $errPath" -ForegroundColor DarkGray
    $proc = Start-Process -FilePath "go" `
        -ArgumentList "run", "./cmd/server" `
        -WorkingDirectory $BackendDir `
        -RedirectStandardOutput $logPath `
        -RedirectStandardError $errPath `
        -PassThru `
        -WindowStyle Hidden

    if (-not (Wait-BackendReady -Port $BackendPort -Process $proc -LogPath $logPath -ErrPath $errPath)) {
        Stop-ProcessTree -ProcessId $proc.Id
        throw "后端在 90 秒内未就绪，请查看 $errPath"
    }

    return @{
        name = "backend"
        processId = $proc.Id
        port      = $BackendPort
        url  = "http://localhost:$BackendPort"
    }
}

function Start-GatewayService {
    if (-not (Test-Path (Join-Path $BackendDir "cmd\file-transfer-gateway"))) {
        throw "未找到 backend/cmd/file-transfer-gateway，请确认仓库路径: $BackendDir"
    }

    Ensure-DevDir
    $logPath = Join-Path $DevDir "gateway-run.log"
    $errPath = Join-Path $DevDir "gateway-run.err.log"
    Remove-Item -LiteralPath $logPath, $errPath -ErrorAction SilentlyContinue

    Write-Host ">> 启动文件传输网关 http://localhost:$GatewayPort" -ForegroundColor Green
    Write-Host "  启动日志: $errPath" -ForegroundColor DarkGray
    $proc = Start-Process -FilePath "go" `
        -ArgumentList "run", "./cmd/file-transfer-gateway" `
        -WorkingDirectory $BackendDir `
        -RedirectStandardOutput $logPath `
        -RedirectStandardError $errPath `
        -PassThru `
        -WindowStyle Hidden

    if (-not (Wait-GatewayReady -Port $GatewayPort -Process $proc -LogPath $logPath -ErrPath $errPath)) {
        Stop-ProcessTree -ProcessId $proc.Id
        throw "文件传输网关在 90 秒内未就绪，请查看 $errPath"
    }

    return @{
        name      = "gateway"
        processId = $proc.Id
        port      = $GatewayPort
        url       = "http://localhost:$GatewayPort"
    }
}

function Start-WorkerService {
    if (-not (Test-Path (Join-Path $BackendDir "cmd\file-worker"))) {
        throw "未找到 backend/cmd/file-worker，请确认仓库路径: $BackendDir"
    }

    Ensure-DevDir
    $logPath = Join-Path $DevDir "worker-run.log"
    $errPath = Join-Path $DevDir "worker-run.err.log"
    Remove-Item -LiteralPath $logPath, $errPath -ErrorAction SilentlyContinue

    Write-Host ">> 启动文件 Worker" -ForegroundColor Green
    Write-Host "  启动日志: $errPath" -ForegroundColor DarkGray
    $proc = Start-Process -FilePath "go" `
        -ArgumentList "run", "./cmd/file-worker" `
        -WorkingDirectory $BackendDir `
        -RedirectStandardOutput $logPath `
        -RedirectStandardError $errPath `
        -PassThru `
        -WindowStyle Hidden

    if (-not (Wait-WorkerReady -Process $proc -LogPath $logPath -ErrPath $errPath)) {
        Stop-ProcessTree -ProcessId $proc.Id
        throw "文件 Worker 在 90 秒内未就绪，请查看 $errPath"
    }

    return @{
        name      = "worker"
        processId = $proc.Id
        port      = 0
        url       = ""
    }
}

function Resolve-PnpmCmd {
    $cmd = Get-Command pnpm.cmd -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }

    $npmDir = Join-Path $env:APPDATA "npm"
    $fallback = Join-Path $npmDir "pnpm.cmd"
    if (Test-Path $fallback) { return $fallback }

    throw "未找到 pnpm.cmd，请先安装: npm install -g pnpm"
}

function Start-FrontendService {
    if (-not (Test-Path (Join-Path $FrontedDir "package.json"))) {
        throw "未找到 fronted/package.json，请确认仓库路径: $FrontedDir"
    }
    $pnpmCmd = Resolve-PnpmCmd

    Write-Host ">> 启动前端 http://localhost:$FrontendPort" -ForegroundColor Green
    $proc = Start-Process -FilePath $pnpmCmd `
        -ArgumentList "dev" `
        -WorkingDirectory $FrontedDir `
        -PassThru `
        -WindowStyle Normal

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

Stop-DevServices -IncludeBackend $stopBackend -IncludeFrontend $stopFrontend

if ($Stop) {
    Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
    Write-Host ">> 已停止。" -ForegroundColor Cyan
    exit 0
}

$started = @()
try {
    if ($runBackend) {
        $started += ,(Start-BackendService)
        $started += ,(Start-GatewayService)
        $started += ,(Start-WorkerService)
    }
    if ($runFrontend) { $started += ,(Start-FrontendService) }
} catch {
    foreach ($entry in $started) {
        Stop-ProcessTree -ProcessId $entry.processId
    }
    throw
}
if ($started.Count -gt 0) {
    $retained = @{}
    if (-not $runBackend) {
        foreach ($name in @("backend", "gateway", "worker")) {
            if ($existingPids.ContainsKey($name)) {
                $retained[$name] = $existingPids[$name]
            }
        }
    }
    if (-not $runFrontend -and $existingPids.ContainsKey("frontend")) {
        $retained["frontend"] = $existingPids["frontend"]
    }
    Save-Pids -Entries $started -Existing $retained
}

Write-Host ""
Write-Host "SparkOps 开发服务已启动：" -ForegroundColor Cyan
foreach ($entry in $started) {
    Write-Host "  $($entry.name): $($entry.url) (PID $($entry.processId))" -ForegroundColor White
}
Write-Host ""
Write-Host "改代码后再次执行: .\scripts\dev-restart.ps1" -ForegroundColor DarkGray
Write-Host "性能对比（生产模式）: .\scripts\dev-restart-prod.ps1" -ForegroundColor DarkGray
