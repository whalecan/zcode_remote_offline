# ZCode 离线资源拉取（有网维护）

本目录用于在**可访问公网 CDN** 的机器上，拉取 / 更新远程开发资源，并写入旁边的 `../offline` 镜像目录。

> 仅处理 **linux-x64**。  
> 离线环境的使用方法见 [`../offline/README.md`](../offline/README.md)。

---

## 目录关系

```text
resources/
├── online/                         ← 本目录（有网维护）
│   ├── README.md
│   ├── go.mod
│   └── main.go                     ← Go 下载工具（唯一实现）
└── offline/                        ← 下载目标 / 离线分发包
    ├── README.md
    ├── main.go / go.mod              ← CDN 服务本体
    ├── Dockerfile / docker-up.sh     ← 可选 Docker 包装
    ├── serve.ps1
    ├── sha256sums-linux-x64.txt
    └── zcode/electron/releases/...
```

---

## 前置条件

- 能访问以下任一 CDN：
  - `https://cdn-zcode.z.ai/zcode/electron/releases`
  - `https://cdn.codegeex.cn/zcode/electron/releases`
  - `https://cdn.zcode-ai.com/zcode/electron/releases`
- 从 [GitHub Releases](https://github.com/coloraven/zcode_remote_offline/releases) 下载 `download-offline-assets_<os>_<arch>[.exe]`（日常使用无需 Go）

---

## 用法（推荐：Release 二进制）

将二进制放在 `online/` 目录下执行（写入同级 `../offline`）：

```powershell
# 列出 CDN 上可用版本（并标注本地是否已有）
.\download-offline-assets_windows_amd64.exe -list

.\download-offline-assets_windows_amd64.exe -version 3.4.2
.\download-offline-assets_windows_amd64.exe -version 3.5.0 -force
.\download-offline-assets_windows_amd64.exe -version 3.4.2 -cdn https://cdn-zcode.z.ai/zcode/electron/releases
.\download-offline-assets_windows_amd64.exe -help
```

```bash
chmod +x download-offline-assets_linux_amd64
./download-offline-assets_linux_amd64 -list
./download-offline-assets_linux_amd64 -version 3.4.2
```

不传 `-version` 时，会尝试使用 `offline` 里已有的最高版本。

> `-list` 会读取 update feed / changelog 收集候选，再探测各版本是否存在 `manifest-linux-x64.json`（CDN 本身无目录索引）。

---

## 开发（可选）

改源码时需要 Go 1.22+：

```powershell
go run . -version 3.4.2
go build -trimpath -ldflags="-s -w" -o download-offline-assets.exe .
.\download-offline-assets.exe -version 3.4.2
```

---

## 工具会做什么

1. 从 CDN 下载 `manifest-linux-x64.json` → `offline/zcode/electron/releases/<version>/`
2. 按 manifest 下载全部组件 `.tar.gz` → `offline/zcode/electron/releases/components/linux-x64/...`
3. 用组件 `sha256` 校验本地文件
4. 更新 `offline/sha256sums-linux-x64.txt`

对应线上路径：

| 资源 | URL 形态 |
|------|----------|
| 清单文件 | `{CDN}/<version>/manifest-linux-x64.json` |
| 组件包 | `{CDN}/components/linux-x64/<id>/<version>.tar.gz` |

---

## 更新后如何分发到离线机

1. 在本机（有网）跑完下载工具
2. 将整个 `offline/` 目录拷贝到离线电脑
3. 在离线机按 [`../offline/README.md`](../offline/README.md) 用 Release 二进制或 Docker 启动本地 CDN，并配置环境变量

多版本可反复执行并指定不同 `-version`；离线服务会按请求路径自动响应，无需改配置。

不必把 `online/` 拷到离线环境（离线机不需要下载工具）。

---

## 常见问题

### 报错「没有可用的 CDN」

检查网络，或用 `-cdn` 指定可达的 releases 根地址。

### 报错「appVersion 不匹配」

CDN 上该版本清单的 `appVersion` 与传入版本不一致，请确认版本号是否写对。

### 只想更新某个版本、保留旧版本文件

直接指定新版本即可；旧版本目录不会被自动删除。若不需要旧包可手动删 `offline/zcode/electron/releases/<旧版本>`。
