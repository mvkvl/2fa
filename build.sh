#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

usage() {
	cat <<'EOF'
Использование: ./build.sh [-o PATH] [--version VERSION]

Собирает 2fa без отладочных символов.

Параметры:
  -o, --output PATH   путь к бинарнику (по умолчанию: 2fa)
      --version STR   версия для встраивания
                      (по умолчанию: git describe --tags --always --dirty)
  -h, --help          показать эту справку

Переменные окружения:
  VERSION             версия, если не указан --version
  GOOS, GOARCH        целевая платформа
  CGO_ENABLED         по умолчанию 0 (сборка без cgo)
EOF
}

OUTPUT="2fa"
VERSION="${VERSION:-}"

while [ $# -gt 0 ]; do
	case "$1" in
		-o|--output)
			[ $# -ge 2 ] || { echo "build.sh: для $1 нужен путь" >&2; exit 2; }
			OUTPUT="$2"
			shift 2
			;;
		--version)
			[ $# -ge 2 ] || { echo "build.sh: для --version нужно значение" >&2; exit 2; }
			VERSION="$2"
			shift 2
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "build.sh: неизвестный аргумент: $1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if ! command -v go >/dev/null 2>&1; then
	echo "build.sh: команда go не найдена в PATH" >&2
	exit 1
fi

if [ -z "$VERSION" ]; then
	if command -v git >/dev/null 2>&1 && git rev-parse --git-dir >/dev/null 2>&1; then
		VERSION="$(git describe --tags --always --dirty 2>/dev/null || true)"
	fi
	[ -n "$VERSION" ] || VERSION="dev"
fi

mkdir -p "$(dirname -- "$OUTPUT")"

# -s и -w убирают таблицу символов и DWARF, -trimpath — локальные пути,
# -buildid= — идентификатор сборки. Версия подставляется в main.version.
LDFLAGS="-s -w -buildid= -X main.version=${VERSION}"
CGO_ENABLED="${CGO_ENABLED:-0}"
export CGO_ENABLED

echo "Сборка 2fa ${VERSION} -> ${OUTPUT}"
go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$OUTPUT" .

if [ -f "$OUTPUT" ]; then
	printf 'Размер: %s\n' "$(wc -c <"$OUTPUT" | awk '{printf "%.1f MiB", $1/1048576}')"
fi
