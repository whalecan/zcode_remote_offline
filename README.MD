# zcode_remote_offline

| 目录 | 用途 |
|------|------|
| [`online/`](online/) | 有网环境：Go 工具从公网 CDN 拉取 / 更新资源到 `offline/` |
| [`offline/`](offline/) | 离线环境：Go CDN 服务MOCK线上资源源（Docker 可选） |

有网拉取：

```powershell
cd online
go run . -version 3.4.2
```

离线提供服务（多版本自动响应）：

```powershell
cd offline
go run .
```

## Release 产物

推送 `v*` 标签（或手动运行 Actions → Release）后，GitHub Release 会发布：

| 产物 | 平台 |
|------|------|
| `zcode-remote-server-downloader_<os>_<arch>[.exe]` | linux/windows × amd64/arm64 |
| `download-offline-assets_<os>_<arch>[.exe]` | 同上 |
| `zcode-remote-server-downloader-docker-linux-amd64.tar.gz` | Linux amd64 Docker 包 |
| `zcode-remote-server-downloader-docker-linux-arm64.tar.gz` | Linux arm64 Docker 包 |
| `SHA256SUMS.txt` | 校验 |

详见 [`offline/README.md`](offline/README.md)、[`online/README.md`](online/README.md)。
