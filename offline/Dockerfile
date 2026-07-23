# 多架构构建（linux/amd64、linux/arm64）
# 需要 BuildKit：
#   docker buildx build --platform linux/amd64 -t zcode-remote-server-downloader:amd64 .
#   docker buildx build --platform linux/arm64 -t zcode-remote-server-downloader:arm64 .
# 或由 docker-up.sh / compose 自动选择当前架构。
#
# 不依赖 Docker 时可直接：go run .

ARG TARGETOS=linux
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /cdn .

FROM scratch
COPY --from=build /cdn /cdn

ENV CDN_ROOT=/data
ENV PORT=80
ENV BIND=0.0.0.0

EXPOSE 80
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/cdn", "-healthcheck"]

ENTRYPOINT ["/cdn"]
