#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

usage() {
	cat <<'EOF'
Использование: ./build-gui.sh [-o PATH] [--version VERSION]

Собирает GUI-приложение 2fa на Wails без отладочных символов.
Сборка выполняется для текущей платформы; для упаковки .app/.deb/.rpm
используйте wails3 package.

Параметры:
  -o, --output PATH   путь к бинарнику (по умолчанию: gui/build/bin/2fa)
      --version STR   версия для встраивания
                      (по умолчанию: git describe --tags --always --dirty)
  -h, --help          показать эту справку

Переменные окружения:
  VERSION             версия, если не указан --version
  GOOS, GOARCH        целевая платформа
  CGO_ENABLED         по умолчанию 1 (0 для Windows)
EOF
}

OUTPUT="gui/build/bin/2fa"
VERSION="${VERSION:-}"

while [ $# -gt 0 ]; do
	case "$1" in
		-o|--output)
			[ $# -ge 2 ] || { echo "build-gui.sh: для $1 нужен путь" >&2; exit 2; }
			OUTPUT="$2"
			shift 2
			;;
		--version)
			[ $# -ge 2 ] || { echo "build-gui.sh: для --version нужно значение" >&2; exit 2; }
			VERSION="$2"
			shift 2
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "build-gui.sh: неизвестный аргумент: $1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if ! command -v go >/dev/null 2>&1; then
	echo "build-gui.sh: команда go не найдена в PATH" >&2
	exit 1
fi

if [ -z "$VERSION" ]; then
	if command -v git >/dev/null 2>&1 && git rev-parse --git-dir >/dev/null 2>&1; then
		VERSION="$(git describe --tags --always --dirty 2>/dev/null || true)"
	fi
	[ -n "$VERSION" ] || VERSION="dev"
fi

GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
if [ -z "${CGO_ENABLED:-}" ]; then
	if [ "$GOOS" = "windows" ]; then
		CGO_ENABLED=0
	else
		CGO_ENABLED=1
	fi
fi
export GOOS GOARCH CGO_ENABLED

if [ "$GOOS" = "linux" ]; then
	if ! command -v pkg-config >/dev/null 2>&1; then
		echo "build-gui.sh: не найден pkg-config (нужен для GTK4/WebKitGTK)" >&2
		exit 1
	fi
	if ! pkg-config --exists gtk4; then
		echo "build-gui.sh: не найдены заголовки GTK4 (установите libgtk-4-dev)" >&2
		exit 1
	fi
	if ! pkg-config --exists webkitgtk-6.0; then
		echo "build-gui.sh: не найдены заголовки WebKitGTK 6.0 (установите libwebkitgtk-6.0-dev)" >&2
		exit 1
	fi
fi

if [[ "$OUTPUT" != /* ]]; then
	OUTPUT="$SCRIPT_DIR/$OUTPUT"
fi
if [ "$GOOS" = "windows" ] && [[ "$OUTPUT" != *.exe ]]; then
	OUTPUT="${OUTPUT}.exe"
fi
mkdir -p "$(dirname -- "$OUTPUT")"

LDFLAGS="-s -w -buildid= -X main.version=${VERSION}"
if [ "$GOOS" = "windows" ]; then
	LDFLAGS="$LDFLAGS -H windowsgui"
fi

echo "Сборка GUI 2fa ${VERSION} (${GOOS}/${GOARCH}) -> ${OUTPUT}"
cd "$SCRIPT_DIR/gui"
go build -tags production -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$OUTPUT" .

if [ -f "$OUTPUT" ]; then
	printf 'Размер: %s\n' "$(wc -c <"$OUTPUT" | awk '{printf "%.1f MiB", $1/1048576}')"
fi
