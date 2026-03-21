#!/usr/bin/env bash
# build.sh — 跨平台构建打包脚本 | Cross-platform build & package script
# 用法 Usage: bash build.sh [all|windows|linux|darwin]
set -euo pipefail

APP_NAME="go-port-forward"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
OUTPUT_DIR="dist"

LDFLAGS="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

# 目标平台列表 | Target platforms
PLATFORMS=(
    "windows/amd64"
    "windows/arm64"
    "linux/amd64"
    "linux/arm64"
    "linux/arm"
    "darwin/amd64"
    "darwin/arm64"
)

log() { echo -e "\033[1;36m==>\033[0m $*"; }
err() { echo -e "\033[1;31m==>\033[0m $*" >&2; }

build_one() {
    local os="$1" arch="$2"
    local ext="" bin_name="${APP_NAME}"
    [[ "$os" == "windows" ]] && ext=".exe"
    bin_name="${APP_NAME}${ext}"

    local out_dir="${OUTPUT_DIR}/${APP_NAME}-${VERSION}-${os}-${arch}"
    mkdir -p "$out_dir"

    # amd64 使用 sonic/base64x (依赖 x86 指令集)，其他架构使用 go_json 回退
    # amd64 uses sonic/base64x (x86 instruction set), other archs fall back to go_json
    local tags
    if [[ "$arch" == "amd64" ]]; then
        tags="base64x sonic"
    else
        tags="go_json"
    fi

    log "Building ${os}/${arch} (tags: ${tags}) ..."
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
        go build -trimpath -tags "$tags" -ldflags "$LDFLAGS" -o "${out_dir}/${bin_name}" .

    # 复制配置示例和 LICENSE | Copy sample config & LICENSE
    [[ -f config.yaml ]] && cp config.yaml "${out_dir}/config.yaml.example"
    [[ -f LICENSE ]] && cp LICENSE "${out_dir}/LICENSE"

    # 打包 | Package (使用子 shell 避免 cd 污染当前目录)
    local archive
    local dirname="${APP_NAME}-${VERSION}-${os}-${arch}"
    (
        cd "$OUTPUT_DIR"
        if [[ "$os" == "windows" ]]; then
            archive="${dirname}.zip"
            zip -qr "$archive" "$dirname"
        else
            archive="${dirname}.tar.gz"
            tar -czf "$archive" "$dirname"
        fi
    )

    log "Packaged: ${OUTPUT_DIR}/${archive}"
}

build_targets() {
    local filter="${1:-all}"
    for platform in "${PLATFORMS[@]}"; do
        local os="${platform%/*}"
        local arch="${platform#*/}"
        if [[ "$filter" == "all" || "$filter" == "$os" ]]; then
            build_one "$os" "$arch"
        fi
    done
}

clean() {
    log "Cleaning ${OUTPUT_DIR}/ ..."
    rm -rf "$OUTPUT_DIR"
}

generate_checksums() {
    log "Generating checksums ..."
    # macOS 没有 sha256sum，回退到 shasum -a 256
    local sha_cmd="sha256sum"
    if ! command -v sha256sum &>/dev/null; then
        sha_cmd="shasum -a 256"
    fi
    (
        cd "$OUTPUT_DIR"
        $sha_cmd *.tar.gz *.zip 2>/dev/null > checksums-sha256.txt || true
    )
    log "Checksums: ${OUTPUT_DIR}/checksums-sha256.txt"
}

main() {
    local target="${1:-all}"

    log "${APP_NAME} build script"
    log "Version: ${VERSION}"
    log "Target:  ${target}"

    clean
    mkdir -p "$OUTPUT_DIR"

    build_targets "$target"
    generate_checksums

    log "Done! All artifacts in ${OUTPUT_DIR}/"
    ls -lh "${OUTPUT_DIR}/"*.tar.gz "${OUTPUT_DIR}/"*.zip 2>/dev/null || true
}

main "$@"

