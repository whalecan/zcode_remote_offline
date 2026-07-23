ZCode Offline CDN — Linux Docker 包（预编译二进制）

1. 准备数据目录（把原 offline 资源放进来，至少包含 zcode/）：
     mkdir -p data
     cp -a /path/to/offline/zcode data/

2. 启动：
     chmod +x docker-up.sh zcode-remote-server-downloader
     ./docker-up.sh

3. 客户端设置：
     ZCODE_REMOTE_ASSET_CDN_BASE_URL=http://<host>:8765/zcode/electron/releases

停止： ./docker-up.sh --down
