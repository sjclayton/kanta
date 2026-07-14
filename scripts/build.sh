#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUT_DIR="dist"
NAME="kanta-${VERSION}-linux-amd64"
SRC="build/bin/kanta"
DEST="${OUT_DIR}/${NAME}/kanta"

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}/${NAME}"

wails build -tags webkit2_41 -clean -trimpath -ldflags "-s -w"

cp "${SRC}" "${DEST}"
cp README.md "${OUT_DIR}/${NAME}/README.md"

(cd "${OUT_DIR}" && sha256sum "${NAME}/kanta" > "${NAME}.sha256")

echo "wrote ${DEST}"
echo "checksum ${OUT_DIR}/${NAME}.sha256"
