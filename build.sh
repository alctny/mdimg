#!/bin/sh
set -eu

cd -- "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

usage() {
    cat <<'EOF'
Usage: build.sh [-l] [-m] [-w] [-amd] [-arm] [-a] [-x]
  -l       Build for Linux
  -m       Build for macOS
  -w       Build for Windows
  -amd     Build for x86_64 (amd64)
  -arm     Build for ARM64 (arm64)
  -a       Build all three operating systems and both architectures
  -x       Compress with UPX after building (disabled by default)
  -h, --help  Show this help

Multiple OS/architecture options build every selected combination.
Omitted OS/architecture options default to the host OS/CPU architecture.
EOF
}

linux=0
macos=0
windows=0
amd64=0
arm64=0
all=0
compress=0
for option in "$@"; do
    case "$option" in
        -l) linux=1 ;;
        -m) macos=1 ;;
        -w) windows=1 ;;
        -amd) amd64=1 ;;
        -arm) arm64=1 ;;
        -a) all=1 ;;
        -x) compress=1 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "Error: Unknown option: $option" >&2; usage >&2; exit 1 ;;
    esac
done

if ! command -v go >/dev/null 2>&1; then
    echo "Error: Go 1.23 or newer must be installed and available on PATH." >&2
    exit 1
fi

target_oses=
target_arches=
if [ "$all" -eq 1 ]; then
    target_oses="darwin linux windows"
    target_arches="amd64 arm64"
else
    if [ "$macos" -eq 1 ]; then target_oses="$target_oses darwin"; fi
    if [ "$linux" -eq 1 ]; then target_oses="$target_oses linux"; fi
    if [ "$windows" -eq 1 ]; then target_oses="$target_oses windows"; fi
    if [ "$amd64" -eq 1 ]; then target_arches="$target_arches amd64"; fi
    if [ "$arm64" -eq 1 ]; then target_arches="$target_arches arm64"; fi
    if [ -z "$target_oses" ]; then target_oses=$(go env GOHOSTOS); fi
    if [ -z "$target_arches" ]; then target_arches=$(go env GOHOSTARCH); fi
fi

upx=
if [ "$compress" -eq 1 ]; then
    if ! upx=$(command -v upx); then
        echo "Error: -x requires UPX to be installed and available on PATH." >&2
        exit 1
    fi
    echo "Using UPX --best --ultra-brute compression."
fi

git_tag=$(git describe --tags --abbrev=0 HEAD 2>/dev/null || printf 'untagged')
git_commit=$(git rev-parse HEAD 2>/dev/null || printf 'unknown')
build_date=$(TZ=Asia/Shanghai date +%Y-%m-%d)
version=$git_tag
if [ "$version" = untagged ]; then
    version=dev
fi
ldflags="-s -w -buildid= -X main.version=$version -X main.buildDate=$build_date -X main.gitCommit=$git_commit -X main.gitTag=$git_tag"

if command -v sha256sum >/dev/null 2>&1; then
    checksum() { sha256sum "$1"; }
else
    checksum() { shasum -a 256 "$1"; }
fi

mkdir -p dist
checksums=
for target_os in $target_oses; do
    for arch in $target_arches; do
        name="mdimg-$target_os-$arch"
        if [ "$target_os" = windows ]; then
            name="$name.exe"
        fi
        path="dist/$name"
        echo "Building $path ($version, $git_commit, $build_date)"
        CGO_ENABLED=0 GOOS="$target_os" GOARCH="$arch" \
            go build -trimpath -buildvcs=false -ldflags="$ldflags" -o "$path" .

        if [ -n "$upx" ]; then
            packed_dir=$(mktemp -d dist/.upx-XXXXXX)
            if "$upx" --best --ultra-brute -o "$packed_dir/$name" "$path" && \
                "$upx" -t "$packed_dir/$name"; then
                mv "$packed_dir/$name" "$path"
            else
                echo "Warning: UPX compression or verification failed for $path; keeping uncompressed binary." >&2
            fi
            rm -rf "$packed_dir"
        fi
        checksums="$checksums$(cd dist && checksum "$name")
"
    done
done
printf '%s' "$checksums" > dist/SHA256SUMS
