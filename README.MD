# ZCode 远程开发包下载MOCK服务
## 项目用途
ZCode 连接 SSH 远程（linux-x64）时，需要在线从 CDN 拉取 `manifest` 与组件包（`server-bundle` / `node-runtime` 等），再部署到远程主机。

本项目解决**离线环境** 的环境中，ZCode 无法连CDN拉取，ssh连接远程服务器进行开发的问题。

本仓库做两件事：
1. **有网侧**：从官方 CDN 把指定版本资源镜像到本地 `offline/` 目录  
2. **离线侧**：用本地 HTTP 服务（或 Docker）MOCK 线上 CDN 路径，让客户端通过环境变量 `ZCODE_REMOTE_ASSET_CDN_BASE_URL` 走内网/本机地址完成下载

支持多版本资源并存，按客户端请求的版本路径自动响应。日常使用下载 [Releases](https://github.com/coloraven/zcode_remote_offline/releases) 二进制即可，无需安装 Go。

## 项目结构

| 目录 | 用途 |
|------|------|
| [`online/`](online/) | 有网环境：从公网 CDN 拉取 / 更新资源到 `offline/` |
| [`offline/`](offline/) | 离线环境：本地 HTTP 服务 MOCK 线上 CDN（多版本自动响应） |

正常使用请从 [GitHub Releases](https://github.com/coloraven/zcode_remote_offline/releases) 下载对应平台二进制（或 Linux Docker 包），无需安装 Go。

---
## 使用方法
### 1. 有网拉取资源

下载 `download-offline-assets_<os>_<arch>[.exe]`，放到仓库旁（与 `offline/` 同级的 `online/` 工作方式：工具会写入 `../offline`）。在含 `offline` 目录的上一级结构中执行更直观：把二进制放进 `online/` 再运行。

```powershell
# Windows 示例（amd64）
.\download-offline-assets_windows_amd64.exe -list
.\download-offline-assets_windows_amd64.exe -version 3.4.2
.\download-offline-assets_windows_amd64.exe -version 3.5.0 -force
```

```bash
# Linux 示例
chmod +x download-offline-assets_linux_amd64
./download-offline-assets_linux_amd64 -list
./download-offline-assets_linux_amd64 -version 3.4.2
```

资源写入 `offline/zcode/electron/releases/...`。

---

### 2. 离线提供 CDN 服务

先准备好已拉取的 `offline/` 资源目录，再任选其一：

#### 方式 A：直接跑二进制（推荐）

从 Release 下载 `zcode-cdn-mock_<os>_<arch>[.exe]`，放到 `offline/` 目录下执行：

```powershell
# Windows
cd offline
.\zcode-cdn-mock_windows_amd64.exe
```

```bash
# Linux
cd offline
chmod +x zcode-cdn-mock_linux_amd64
./zcode-cdn-mock_linux_amd64
```

默认：`http://127.0.0.1:8765`  
CDN 根：`http://127.0.0.1:8765/zcode/electron/releases`

```bash
PORT=9000 BIND=0.0.0.0 ./zcode-cdn-mock_linux_amd64
```

#### 方式 B：Docker（Linux amd64 / arm64）— 仅离线 CDN 服务

Docker **只用于**离线侧的本地 CDN HTTP 服务（把已镜像的 `offline/zcode` 资源对外提供下载）。  
有网侧的 `download-offline-assets_*`（从公网 CDN 拉资源）**不需要**构建镜像，直接跑二进制即可。

离线机往往无法访问镜像仓库、也不宜现场编译。请先在**有 Docker 的联网（或可构建）机器**上，把「离线 CDN 服务」打成镜像并导出，再拷到离线机 `docker load` 后启动。

##### B1. 有网机构建「离线 CDN 服务」镜像并打包

从 Release 下载 `zcode-cdn-mock_*` 与 `offline/Dockerfile.prebuilt`（或解压 `zcode-cdn-mock-docker-linux-*.tar.gz`）：

```bash
# 以 amd64 为例；arm64 将文件名中的 amd64 换成 arm64
cp zcode-cdn-mock_linux_amd64 zcode-cdn-mock
chmod +x zcode-cdn-mock

docker build -f Dockerfile.prebuilt -t zcode-cdn-mock:local .
docker save zcode-cdn-mock:local | gzip > zcode-cdn-mock-image-linux-amd64.tar.gz
```

将下列内容一并拷到离线环境：

| 文件 | 说明 |
|------|------|
| `zcode-cdn-mock-image-linux-amd64.tar.gz` | 已构建好的镜像包 |
| `offline/` 资源目录（至少含 `zcode/`） | CDN 静态资源，挂载进容器 |

##### B2. 离线机加载镜像并启动

```bash
gunzip -c zcode-cdn-mock-image-linux-amd64.tar.gz | docker load

# 数据目录：把已拉取的 offline 资源挂进来（目录内需有 zcode/）
DATA_DIR=/path/to/offline

docker run -d --restart unless-stopped \
  --name zcode-cdn-mock \
  -p 8765:80 \
  -v "${DATA_DIR}:/data:ro" \
  -e CDN_ROOT=/data -e PORT=80 -e BIND=0.0.0.0 \
  zcode-cdn-mock:local
```

停止 / 删除容器：

```bash
docker stop zcode-cdn-mock
docker rm zcode-cdn-mock
```

默认：`http://127.0.0.1:8765`，CDN 根：`http://127.0.0.1:8765/zcode/electron/releases`。

> 说明：Release 里的 `zcode-cdn-mock-docker-linux-*.tar.gz` 是「预编译二进制 + Dockerfile」打包，本机执行 `./docker-up.sh` 仍会**现场 `docker build`**（`FROM scratch`，一般无需拉外网基础镜像）。完全离线、且希望跳过构建时，请按上面 **B1 → B2** 提前导出镜像包。

---

### 3. 配置 ZCode 客户端环境变量

#### Windows（用户级，持久生效）

```powershell
[System.Environment]::SetEnvironmentVariable(
  'ZCODE_REMOTE_ASSET_CDN_BASE_URL',
  'http://127.0.0.1:8765/zcode/electron/releases',
  'User'
)
```

#### Linux / macOS（当前 shell 临时生效）

```bash
export ZCODE_REMOTE_ASSET_CDN_BASE_URL='http://127.0.0.1:8765/zcode/electron/releases'
```

#### Linux / macOS（持久化，bash）

```bash
echo "export ZCODE_REMOTE_ASSET_CDN_BASE_URL='http://127.0.0.1:8765/zcode/electron/releases'" >> ~/.bashrc
```

若 CDN 跑在另一台机器，把地址改成例如 `http://192.168.x.x:8765/zcode/electron/releases`。

然后**完全退出并重启 ZCode**，再进行 SSH 远程连接。

**使用注意：** 在 ZCode 远程连接相关设置中，`资源下载方式` 必须选 **`本地下载后上传`**（由本机经本地 CDN 下载资源，再上传到远程；不要选远程主机直接拉公网 CDN）。
---


## 开发（可选）

改代码时可用源码调试，日常使用不需要：

```powershell
cd online;  go run . -version 3.4.2
cd offline; go run .
```

详见 [`offline/README.md`](offline/README.md)、[`online/README.md`](online/README.md)。

## Release 产物一览

| 产物 | 说明 |
|------|------|
| `download-offline-assets_<os>_<arch>[.exe]` | 有网拉取工具 |
| `zcode-cdn-mock_<os>_<arch>[.exe]` | 离线 CDN 服务 |
| `zcode-cdn-mock-docker-linux-amd64.tar.gz` | Linux amd64 Docker 包 |
| `zcode-cdn-mock-docker-linux-arm64.tar.gz` | Linux arm64 Docker 包 |
| `SHA256SUMS.txt` | 校验 |

平台：linux / windows × amd64 / arm64。
---
