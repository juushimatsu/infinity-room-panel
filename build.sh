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

  SUFFIX=""
  [ "$OS" = "windows" ] && SUFFIX=".exe"

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
    || { echo "  WARNING: electron-packager failed for $OS/$ARCH (binary still ships)"; continue; }

  # electron-packager creates a single subdir like audiobot-panel-electron-<EOS>-<EARCH>/
  # Copy backend binary + frontend/build into that app's resources/ so Electron finds them
  # (electron/main.js looks for backend at process.resourcesPath/backend/<binary>
  #  and Go backend resolves frontend at <projectRoot>/frontend/build where projectRoot = resources dir)
  INNER_APP="$(find "$DIST_PATH/electron-app" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
  if [ -z "$INNER_APP" ] || [ ! -d "$INNER_APP/resources" ]; then
    echo "  WARNING: could not locate Electron app resources/ for $OS/$ARCH"
    continue
  fi

  mkdir -p "$INNER_APP/resources/backend"
  cp "$DIST_PATH/audiobot-panel${SUFFIX}" "$INNER_APP/resources/backend/audiobot-panel${SUFFIX}"
  cp -r "$PROJECT_ROOT/frontend/build" "$INNER_APP/resources/frontend-build-tmp"
  rm -rf "$INNER_APP/resources/frontend"
  mkdir -p "$INNER_APP/resources/frontend"
  mv "$INNER_APP/resources/frontend-build-tmp" "$INNER_APP/resources/frontend/build"
  echo "    backend + frontend embedded in $INNER_APP"
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
