#!/usr/bin/env sh
# 使用 Release 预编译二进制 + Dockerfile.prebuilt 启动（适合离线机无 Go）
#
# 用法（在含 zcode-remote-server-downloader 二进制的目录）：
#   ./docker-up-prebuilt.sh
#   ./docker-up-prebuilt.sh --amd64
#   ./docker-up-prebuilt.sh --arm64
#   DATA_DIR=/path/to/offline-data ./docker-up-prebuilt.sh
#   ./docker-up-prebuilt.sh --down

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
      sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'
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
  echo "未找到 docker。" >&2
  exit 1
fi

if [ "$DOWN" -eq 1 ]; then
  echo "停止离线 CDN..."
  if [ -f docker-compose.prebuilt.yml ]; then
    docker compose -f docker-compose.prebuilt.yml down
  else
    docker compose down
  fi
  exit 0
fi

if [ ! -f ./zcode-remote-server-downloader ]; then
  echo "缺少 ./zcode-remote-server-downloader 二进制。请从 GitHub Release 下载对应架构文件并改名为 zcode-remote-server-downloader。" >&2
  exit 1
fi

# Release 包内文件名为 docker-compose.yml；仓库内为 docker-compose.prebuilt.yml
COMPOSE_FILE=docker-compose.yml
if [ -f docker-compose.prebuilt.yml ]; then
  COMPOSE_FILE=docker-compose.prebuilt.yml
fi

if [ ! -f ./Dockerfile.prebuilt ]; then
  echo "缺少 Dockerfile.prebuilt。" >&2
  exit 1
fi

if [ -z "$PLATFORM" ]; then
  PLATFORM=$(detect_platform)
fi
export DOCKER_PLATFORM="$PLATFORM"
PORT="${PORT:-8765}"
export PORT
export DATA_DIR="${DATA_DIR:-./data}"

echo "启动 ZCode 离线 CDN（预编译 / ${DOCKER_PLATFORM}，端口 ${PORT}）..."
echo "  数据目录   : ${DATA_DIR}"
docker compose -f "$COMPOSE_FILE" up -d --build

BASE="http://127.0.0.1:${PORT}/zcode/electron/releases"
echo ""
echo "已启动。"
echo "  平台       : ${DOCKER_PLATFORM}"
echo "  CDN 根     : ${BASE}"
echo "停止： $0 --down"
