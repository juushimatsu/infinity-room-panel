#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="$PROJECT_ROOT/dist"

echo "=== Building audiobot-panel v$VERSION ==="

# Strip leading 'v' for tools that require pure numeric versions (rcedit on Windows)
NUMERIC_VERSION="${VERSION#v}"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# 1. Frontend (Preact + Vite)
echo "[1/4] Building frontend..."
cd "$PROJECT_ROOT/frontend"
npm ci
npm run build
echo "  -> frontend/build/ ($(du -sh build | cut -f1))"

# 2. Go backend, cross-compiled
echo "[2/4] Building Go backends..."
cd "$PROJECT_ROOT"

TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%%/*}"
  ARCH="${TARGET##*/}"
  SUFFIX=""
  [ "$OS" = "windows" ] && SUFFIX=".exe"

  echo "  -> $OS/$ARCH..."
  DIST_NAME="audiobot-panel-${OS}-${ARCH}"
  mkdir -p "$DIST_DIR/$DIST_NAME"

  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build \
    -ldflags="-s -w" \
    -o "$DIST_DIR/$DIST_NAME/audiobot-panel${SUFFIX}" \
    ./backend/

  cp -r "$PROJECT_ROOT/frontend/build" \
    "$DIST_DIR/$DIST_NAME/frontend-build"
done

# 3. Electron packaging (best-effort; cross-platform from Windows is limited)
echo "[3/4] Packaging Electron apps (best-effort)..."
cd "$PROJECT_ROOT/electron"
npm ci

for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%%/*}"
  ARCH="${TARGET##*/}"
  DIST_NAME="audiobot-panel-${OS}-${ARCH}"
  DIST_PATH="$DIST_DIR/$DIST_NAME"

  # electron-packager naming: amd64 -> x64, windows -> win32
  EARCH="$ARCH"
  [ "$ARCH" = "amd64" ] && EARCH="x64"
  EOS="$OS"
  [ "$OS" = "windows" ] && EOS="win32"

  echo "  -> $OS/$ARCH electron-packager (platform=$EOS arch=$EARCH)..."
  npx --yes electron-packager . \
    --platform="$EOS" \
    --arch="$EARCH" \
    --out="$DIST_PATH/electron-app" \
    --overwrite \
    --asar \
    --app-version="$NUMERIC_VERSION" \
    --name="audiobot-panel-electron" \
    --executable-name="audiobot-panel-electron" \
    || echo "  WARNING: electron-packager failed for $OS/$ARCH (binary still ships)"
done

cd "$PROJECT_ROOT"

# 4. Archives
echo "[4/4] Creating archives..."
cd "$DIST_DIR"

for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%%/*}"
  ARCH="${TARGET##*/}"
  DIST_NAME="audiobot-panel-${OS}-${ARCH}"

  if [ "$OS" = "windows" ]; then
    if command -v zip >/dev/null 2>&1; then
      zip -qr "${DIST_NAME}.zip" "$DIST_NAME"
      echo "  -> ${DIST_NAME}.zip"
    else
      # Fallback: PowerShell Compress-Archive on Windows hosts without zip
      powershell -NoProfile -Command "Compress-Archive -Path '$DIST_NAME' -DestinationPath '${DIST_NAME}.zip' -Force"
      echo "  -> ${DIST_NAME}.zip (Compress-Archive)"
    fi
  else
    tar czf "${DIST_NAME}.tar.gz" "$DIST_NAME"
    echo "  -> ${DIST_NAME}.tar.gz"
  fi
done

echo ""
echo "=== Build complete ==="
ls -lh "$DIST_DIR"/*.tar.gz "$DIST_DIR"/*.zip 2>/dev/null || echo "(no archives)"
