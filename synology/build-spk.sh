#!/bin/sh

set -eu

usage() {
	cat <<'EOF'
Usage: ./synology/build-spk.sh [options]

Options:
  --arch <value>        Synology arch or platform value. Default: x86_64
  --version <value>     Package version in DSM format. Default: 1.0.0-0001
  --port <value>        Application port exposed in DSM. Default: 3000
  --maintainer <value>  Maintainer string for INFO. Default: q000q000
  --output-dir <path>   Output directory. Default: dist/synology
  --help                Show this message

Environment variables:
  SPK_ARCH, SPK_VERSION, SPK_PORT, SPK_MAINTAINER, SPK_OUTPUT_DIR, SPK_WORK_DIR
EOF
}

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

PACKAGE_NAME="openspeedtestx"
DISPLAY_NAME="OpenSpeedTestX"
PACKAGE_ARCH="${SPK_ARCH:-x86_64}"
PACKAGE_VERSION="${SPK_VERSION:-1.0.0-0001}"
PACKAGE_PORT="${SPK_PORT:-3000}"
PACKAGE_MAINTAINER="${SPK_MAINTAINER:-q000q000}"
OUTPUT_DIR="${SPK_OUTPUT_DIR:-$REPO_ROOT/dist/synology}"
WORK_DIR="${SPK_WORK_DIR:-$REPO_ROOT/.dist/synology}"
GO_CACHE_DIR="${SPK_GO_CACHE_DIR:-$WORK_DIR/go-cache}"

while [ "$#" -gt 0 ]; do
	case "$1" in
		--arch)
			PACKAGE_ARCH="$2"
			shift 2
			;;
		--version)
			PACKAGE_VERSION="$2"
			shift 2
			;;
		--port)
			PACKAGE_PORT="$2"
			shift 2
			;;
		--maintainer)
			PACKAGE_MAINTAINER="$2"
			shift 2
			;;
		--output-dir)
			OUTPUT_DIR="$2"
			shift 2
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			echo "Unknown option: $1" >&2
			usage >&2
			exit 1
			;;
	esac
done

GOOS=linux
GOARCH=
GOARM=

case "$PACKAGE_ARCH" in
	x86_64|apollolake|avoton|braswell|broadwell|broadwellnk|broadwellntb|broadwellntbap|bromolow|cedarview|coffeelake|denverton|geminilake|grantley|kvmx64|purley|skylaked|v1000)
		GOARCH=amd64
		;;
	i686|evansport)
		GOARCH=386
		;;
	armv7|alpine|alpine4k)
		GOARCH=arm
		GOARM=7
		;;
	armv5|628x)
		GOARCH=arm
		GOARM=5
		;;
	armv8|armada37xx|rtd1296|rtd1619|rtd1619b)
		GOARCH=arm64
		;;
	*)
		echo "Unsupported Synology arch/platform: $PACKAGE_ARCH" >&2
		echo "Use one of: x86_64, armv8, armv7, armv5, i686 or a mapped platform alias." >&2
		exit 1
		;;
esac

PKG_STAGE="$WORK_DIR/package-root"
SPK_STAGE="$WORK_DIR/spk-root"
APP_STAGE="$PKG_STAGE/share/openspeedtestx"
BINARY_PATH="$PKG_STAGE/bin/openspeedtestx"

rm -rf "$WORK_DIR"
mkdir -p "$PKG_STAGE/bin" "$APP_STAGE" "$SPK_STAGE" "$OUTPUT_DIR" "$GO_CACHE_DIR"

echo "Building OpenSpeedTestX for Synology arch: $PACKAGE_ARCH"

if [ -n "$GOARM" ]; then
	env GOCACHE="$GO_CACHE_DIR" CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" GOARM="$GOARM" \
		go build -trimpath -ldflags="-s -w" -o "$BINARY_PATH" ./cmd/server
else
	env GOCACHE="$GO_CACHE_DIR" CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
		go build -trimpath -ldflags="-s -w" -o "$BINARY_PATH" ./cmd/server
fi

cp "$REPO_ROOT/index.html" "$APP_STAGE/"
cp "$REPO_ROOT/hosted.html" "$APP_STAGE/"
cp "$REPO_ROOT/downloading" "$APP_STAGE/"
cp "$REPO_ROOT/upload" "$APP_STAGE/"
cp "$REPO_ROOT/License.md" "$APP_STAGE/"
cp "$REPO_ROOT/README.md" "$APP_STAGE/"
cp -R "$REPO_ROOT/assets" "$APP_STAGE/"

PACKAGE_NAME="$PACKAGE_NAME" \
DISPLAY_NAME="$DISPLAY_NAME" \
PACKAGE_VERSION="$PACKAGE_VERSION" \
PACKAGE_ARCH="$PACKAGE_ARCH" \
PACKAGE_PORT="$PACKAGE_PORT" \
PACKAGE_MAINTAINER="$PACKAGE_MAINTAINER" \
"$SCRIPT_DIR/INFO.sh" > "$SPK_STAGE/INFO"

cp -R "$SCRIPT_DIR/conf" "$SPK_STAGE/conf"
cp -R "$SCRIPT_DIR/scripts" "$SPK_STAGE/scripts"
cp "$SCRIPT_DIR/LICENSE" "$SPK_STAGE/LICENSE"
cp "$SCRIPT_DIR/PACKAGE_ICON.PNG" "$SPK_STAGE/PACKAGE_ICON.PNG"
cp "$SCRIPT_DIR/PACKAGE_ICON_256.PNG" "$SPK_STAGE/PACKAGE_ICON_256.PNG"

tar -C "$PKG_STAGE" -czf "$SPK_STAGE/package.tgz" .

SPK_FILE="${PACKAGE_NAME}-${PACKAGE_ARCH}-${PACKAGE_VERSION}.spk"
tar -C "$SPK_STAGE" -cf "$OUTPUT_DIR/$SPK_FILE" \
	INFO \
	package.tgz \
	scripts \
	conf \
	LICENSE \
	PACKAGE_ICON.PNG \
	PACKAGE_ICON_256.PNG

echo "Created $OUTPUT_DIR/$SPK_FILE"
