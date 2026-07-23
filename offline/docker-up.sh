#!/usr/bin/env sh
# 可选：用 Docker 包装跑 offline 目录下的 Go CDN（非必需，优先直接 go run .）
#
# 用法：
#   ./docker-up.sh                 # 自动检测本机架构（amd64 / arm64）
#   ./docker-up.sh --amd64
#   ./docker-up.sh --arm64
#   PORT=9000 ./docker-up.sh
#   ./docker-up.sh --down

set -eu
cd "$(dirname "$0")"

detect_platform() {
  m=$(uname -m 2>/dev/null || echo amd64)
  case "$m" in
    x86_64|amd64) echo "linux/amd64" ;;
    aarch64|arm64) echo "linux/arm64" ;;
    *)
      echo "无法识别架构 '$m'，请显式传入 --amd64 或 --arm64" >&2
      exit 1
      ;;
  esac
}

PLATFORM="${DOCKER_PLATFORM:-}"
DOWN=0

while [ $# -gt 0 ]; do
  case "$1" in
    --down|-d) DOWN=1 ;;
    --amd64|--x64) PLATFORM=linux/amd64 ;;
    --arm64|--aarch64) PLATFORM=linux/arm64 ;;
    -h|--help)
      sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      exit 1
      ;;
  esac
  shift
done

if ! command -v docker >/dev/null 2>&1; then
  echo "未找到 docker。可直接运行：go run ." >&2
  exit 1
fi

if [ "$DOWN" -eq 1 ]; then
  echo "停止离线 CDN..."
  docker compose down
  exit 0
fi

if [ -z "$PLATFORM" ]; then
  PLATFORM=$(detect_platform)
fi
export DOCKER_PLATFORM="$PLATFORM"

PORT="${PORT:-8765}"
export PORT

echo "启动 ZCode 离线 CDN（Docker ${DOCKER_PLATFORM}，端口 ${PORT}）..."
docker compose up -d --build

BASE="http://127.0.0.1:${PORT}/zcode/electron/releases"
echo ""
echo "已启动。"
echo "  平台       : ${DOCKER_PLATFORM}"
echo "  首页       : http://127.0.0.1:${PORT}/"
echo "  版本列表   : http://127.0.0.1:${PORT}/versions.json"
echo "  CDN 根     : ${BASE}"
echo ""
echo "请设置环境变量后重启 ZCode："
echo "  ZCODE_REMOTE_ASSET_CDN_BASE_URL=${BASE}"
echo ""
echo "停止： ./docker-up.sh --down"
echo "提示：不依赖 Docker 时可直接 go run ."
