#!/usr/bin/env bash
# 一键重启：停止旧进程 → 编译前后端 → 启动后端/Caddy/固定隧道
# 用法：改完代码后在项目根目录执行  ./restart.sh
# 公网地址固定为 https://scm.rockyjiang.org（cloudflared 命名隧道，重启不变）
set -uo pipefail
cd "$(dirname "$0")"

ROOT="$(pwd)"
LOG_DIR="$ROOT/logs"
mkdir -p "$LOG_DIR"

CLOUDFLARED="$(command -v cloudflared || echo /opt/homebrew/opt/cloudflared/bin/cloudflared)"
CADDY="$(command -v caddy || echo /opt/homebrew/opt/caddy/bin/caddy)"

# 1) 停止旧进程（按端口 + 按进程名，精确清理）
echo "==> 1/5 停止旧进程"
kill_port() {
  local pid
  pid=$(lsof -ti tcp:"$1" -sTCP:LISTEN 2>/dev/null || true)
  if [ -n "$pid" ]; then
    echo "    停止端口 $1 (pid $pid)"
    kill "$pid" 2>/dev/null || true
  fi
}
kill_port 8088
kill_port 9090
if pkill -f "cloudflared tunnel" 2>/dev/null; then
  echo "    停止 cloudflared 隧道"
fi
sleep 2

# 2) 编译后端
echo "==> 2/5 编译后端 (go build)"
go build -o bin/server ./cmd/server

# 3) 构建前端（静态产物 web/dist）
echo "==> 3/5 构建前端 (npm run build)"
(cd web && npm run build)

# 4) 启动服务（nohup 脱离终端，日志落 logs/）
echo "==> 4/5 启动服务"
set -a; source .env 2>/dev/null || true; set +a
nohup "$ROOT/bin/server" > "$LOG_DIR/server.log" 2>&1 &
nohup "$CADDY" run --config "$ROOT/Caddyfile" --adapter caddyfile > "$LOG_DIR/caddy.log" 2>&1 &
nohup "$CLOUDFLARED" tunnel run scm > "$LOG_DIR/tunnel.log" 2>&1 &

# 5) 等待隧道就绪
echo "==> 5/5 等待隧道连接"
for _ in $(seq 1 30); do
  if grep -q "Registered tunnel connection" "$LOG_DIR/tunnel.log" 2>/dev/null; then
    break
  fi
  sleep 1
done

echo ""
echo "✅ 重启完成"
echo "   本地:  http://localhost:9090"
echo "   后端:  http://localhost:8088"
echo "   公网:  https://scm.rockyjiang.org（固定）"
