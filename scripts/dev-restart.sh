#!/usr/bin/env bash
# SparkOps 本地开发：停止并重启前后端
# 用法:
#   ./scripts/dev-restart.sh
#   ./scripts/dev-restart.sh --stop
#   ./scripts/dev-restart.sh --backend
#   ./scripts/dev-restart.sh --frontend

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND_DIR="$ROOT/backend"
FRONTED_DIR="$ROOT/fronted"
DEV_DIR="$ROOT/.dev"
PID_FILE="$DEV_DIR/dev.pids.json"
BACKEND_PORT=8080
FRONTEND_PORT=3000

RUN_BACKEND=1
RUN_FRONTEND=1
STOP_ONLY=0

for arg in "$@"; do
  case "$arg" in
    --stop) STOP_ONLY=1 ;;
    --backend) RUN_FRONTEND=0 ;;
    --frontend) RUN_BACKEND=0 ;;
    -h|--help)
      sed -n '2,7p' "$0"
      exit 0
      ;;
    *)
      echo "未知参数: $arg" >&2
      exit 1
      ;;
  esac
done

mkdir -p "$DEV_DIR"

stop_port() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    local pids
    pids="$(lsof -ti tcp:"$port" -sTCP:LISTEN 2>/dev/null || true)"
    if [[ -n "$pids" ]]; then
      echo "  停止端口 $port 上的进程: $pids"
      kill -9 $pids 2>/dev/null || true
    fi
    return
  fi
  if command -v fuser >/dev/null 2>&1; then
    fuser -k "${port}/tcp" 2>/dev/null || true
  fi
}

stop_saved() {
  if [[ -f "$PID_FILE" ]]; then
    python3 - <<'PY' 2>/dev/null || true
import json, os, signal
path = os.environ["PID_FILE"]
try:
    data = json.load(open(path, encoding="utf-8"))
except Exception:
    raise SystemExit(0)
for name in ("backend", "frontend"):
    item = data.get(name) or {}
    pid = int(item.get("pid") or 0)
    if pid > 0:
        try:
            os.kill(pid, signal.SIGTERM)
            print(f"  停止 {name} (PID={pid})")
        except ProcessLookupError:
            pass
PY
    rm -f "$PID_FILE"
  fi
}

echo ">> 停止开发服务..."
export PID_FILE
stop_saved
[[ "$RUN_BACKEND" -eq 1 ]] && stop_port "$BACKEND_PORT"
[[ "$RUN_FRONTEND" -eq 1 ]] && stop_port "$FRONTEND_PORT"
sleep 0.5

if [[ "$STOP_ONLY" -eq 1 ]]; then
  echo ">> 已停止。"
  exit 0
fi

start_backend() {
  command -v go >/dev/null || { echo "未找到 go" >&2; exit 1; }
  [[ -f "$BACKEND_DIR/config.yaml" ]] || echo "  提示: 请创建 backend/config.yaml"
  echo ">> 启动后端 http://localhost:$BACKEND_PORT"
  (cd "$BACKEND_DIR" && go run ./cmd/server) &
  echo $! > "$DEV_DIR/backend.pid"
}

start_frontend() {
  command -v pnpm >/dev/null || { echo "未找到 pnpm" >&2; exit 1; }
  echo ">> 启动前端 http://localhost:$FRONTEND_PORT"
  (cd "$FRONTED_DIR" && pnpm dev) &
  echo $! > "$DEV_DIR/frontend.pid"
}

BACKEND_PID=0
FRONTEND_PID=0

[[ "$RUN_BACKEND" -eq 1 ]] && start_backend && BACKEND_PID="$(cat "$DEV_DIR/backend.pid")"
[[ "$RUN_FRONTEND" -eq 1 ]] && start_frontend && FRONTEND_PID="$(cat "$DEV_DIR/frontend.pid")"

cat > "$PID_FILE" <<EOF
{
  "backend": {"pid": $BACKEND_PID, "port": $BACKEND_PORT, "url": "http://localhost:$BACKEND_PORT"},
  "frontend": {"pid": $FRONTEND_PID, "port": $FRONTEND_PORT, "url": "http://localhost:$FRONTEND_PORT"}
}
EOF

echo ""
echo "SparkOps 开发服务已启动（后台进程）。"
echo "  后端: http://localhost:$BACKEND_PORT"
echo "  前端: http://localhost:$FRONTEND_PORT"
echo ""
echo "改代码后再次执行: ./scripts/dev-restart.sh"
