# План миграции: React+CRA → Preact+Vite, сборка десктоп-бинарников, релиз через gh

> **Цель**: Заменить тяжёлый React+CRA фронтенд на лёгкий Preact+Vite, собрать десктоп-приложения (Electron) под Linux amd64/arm64 и Windows, создать GitHub Release.

---

## Контекст: текущее состояние

| Метрика | Сейчас (React+CRA) |
|---------|-------------------|
| node_modules | 289 MB |
| JS бандл (несжатый) | 154 KB |
| JS бандл (gzip) | ~40 KB |
| CSS | 8 KB |
| RAM при сборке | 500+ MB (webpack) |
| Время сборки на ARM | 30-60 сек |
| Electron | v30, отдельная директория `electron/` |
| Go бинарник | 30 MB, встраивается в Electron |

**Проблемы на ARM**: webpack жрёт RAM, node_modules 289 MB на eMMC/microSD, CRA устарел.

---

## Шаг 1: Инициализация Preact+Vite проекта

### 1.1. Удалить CRA-зависимости

```bash
cd infinity-room-panel/frontend
rm -rf node_modules package-lock.json build
```

### 1.2. Новый `package.json`

```json
{
  "name": "audiobot-panel",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "preact": "^10.25.0"
  },
  "devDependencies": {
    "@preact/preset-vite": "^2.10.0",
    "vite": "^6.3.0",
    "typescript": "^5.8.0"
  }
}
```

**Ожидаемый размер node_modules**: ~30 MB (вместо 289 MB)

### 1.3. Новый `vite.config.ts`

```ts
import { defineConfig } from "vite";
import preact from "@preact/preset-vite";

export default defineConfig({
  plugins: [preact()],
  build: {
    outDir: "build",
    emptyOutDir: true,
    target: "chrome100", // Electron 30+ Chromium baseline
    minify: "terser",
    terserOptions: {
      compress: { drop_console: true },
    },
    rollupOptions: {
      output: {
        manualChunks: undefined,
        entryFileNames: "assets/[name].js",
        assetFileNames: "assets/[name][extname]",
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
```

### 1.4. Новый `tsconfig.json`

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "jsxImportSource": "preact",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"]
  },
  "include": ["src"]
}
```

Ключ: `"jsxImportSource": "preact"` — TSX будет использовать Preact вместо React.

### 1.5. Новый `index.html` (в корне `frontend/`, не в `public/`)

```html
<!DOCTYPE html>
<html lang="ru">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>AudioBot Panel</title>
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet" />
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/index.tsx"></script>
  </body>
</html>
```

---

## Шаг 2: Переписать компоненты на Preact

### Принцип миграции

Preact API-совместим с React. Миграция каждого компонента:

1. Заменить `import React from "react"` → `import { ... } from "preact/hooks"`
2. Заменить `import { useState, useEffect, ... } from "react"` → `import { useState, useEffect, ... } from "preact/hooks"`
3. Заменить `import ReactDOM from "react-dom"` → `import { render } from "preact"`
4. Заменить `import { ... } from "react"` → `import { ... } from "preact"` (для Component, Ref и т.д.)
5. Убрать `key={...}` где не нужно (Preact менее строгий, но лучше оставить)

### 2.1. `src/index.tsx`

```tsx
import { render } from "preact";
import App from "./App";

render(<App />, document.getElementById("root")!);
```

### 2.2. `src/App.tsx`

```tsx
import { useState, useEffect, useCallback, useRef } from "preact/hooks";
// ... остальное без изменений, JSX тот же
```

Единственное изменение: все `import ... from "react"` и `import ... from "react-dom"` → Preact.

### 2.3. Компоненты (`src/components/*.tsx`)

Каждый компонент:

| Было | Стало |
|------|-------|
| `import React, { useState, useEffect } from "react"` | `import { useState, useEffect } from "preact/hooks"` |
| `import React from "react"` (только для JSX) | Убрать — Preact не требует React in scope с `jsxImportSource` |
| `React.ChangeEvent<HTMLInputElement>` | `JSX.TargetedEvent<HTMLInputElement, Event>` |
| `React.FormEvent` | `JSX.TargetedEvent<HTMLFormElement, Event>` |
| `React.RefObject<T>` | `import { Ref } from "preact"` |

**Количество правок**: ~5 строк на файл × 9 файлов = ~45 строк. Механическая замена.

### 2.4. `src/api/client.ts`

Без изменений — чистый TypeScript без React-зависимостей.

### 2.5. `src/styles.css`

Без изменений. Но нужно импортировать в `index.tsx`:

```tsx
import "./styles.css";
```

---

## Шаг 3: UI по DESIGN.md — тёмная тема

### Принцип

DESIGN.md описывает Notion-style дизайн с тёмной темой. Текущий `styles.css` уже адаптирован для тёмной темы (canvas = #0f1a30, surface = #162240). При переписывании нужно:

1. **Сохранить все CSS-переменные из текущего `styles.css`** — они уже адаптированы для тёмной темы
2. **Проверить соответствие DESIGN.md токенам** — текущие значения уже затемнены (canvas, surface, hairline, ink)

### Карта токенов DESIGN.md → текущая тёмная тема

| DESIGN.md токен | Светлое значение | Тёмное значение (текущее) |
|-----------------|-----------------|--------------------------|
| `canvas` | `#ffffff` | `#0f1a30` |
| `surface` | `#f6f5f4` | `#162240` |
| `surface-soft` | `#fafaf9` | `#1a2848` |
| `hairline` | `#e5e3df` | `#1e3050` |
| `hairline-strong` | `#c8c4be` | `#3a4a60` |
| `ink-deep` | `#000000` | `#ffffff` |
| `ink` | `#1a1a1a` | `#e8e6e3` |
| `charcoal` | `#37352f` | `#c4c0b8` |
| `slate` | `#5d5b54` | `#9b9790` |
| `steel` | `#787671` | `#6b6760` |
| `card-tint-*` | Пастельные | Затемнённые варианты |

**Правило**: все токены из DESIGN.md берутся, но инвертируются/затемняются для тёмной темы. Текущий CSS уже это делает правильно — переписывать цвета не нужно.

### Компоненты UI, которые нужно проверить по DESIGN.md

- **Кнопки**: `{rounded.md}` (8px) — прямоугольные, НЕ pill. Текущий `.btn` использует `--radius-md: 8px` ✅
- **Карточки**: `{rounded.lg}` (12px). Текущий `.card` использует `--radius-lg: 12px` ✅
- **Инпуты**: height 44px, `{rounded.md}`. Текущий `.text-input` — 44px, 8px ✅
- **Шрифт**: Inter (Notion Sans fallback). Подключён через Google Fonts ✅
- **Primary CTA**: `{colors.primary}` (#7c3aed) — purple pill. Текущий `.btn-primary` ✅
- **Touch targets**: 40-44px. Текущие кнопки/инпуты ✅

---

## Шаг 4: Electron — обновление

### 4.1. Обновить `electron/package.json`

```json
{
  "name": "audiobot-panel-electron",
  "version": "1.0.0",
  "main": "main.js",
  "dependencies": {
    "electron": "^30.0.0"
  },
  "scripts": {
    "start": "electron ."
  }
}
```

Без изменений — Electron не зависит от фронтенд-стека. Он просто загружает `http://localhost:PORT`.

### 4.2. `electron/main.js`

Без изменений — уже оптимизирован для ARM (disableHardwareAcceleration, disable-features, etc.).

### 4.3. `electron/preload.js`

Без изменений.

---

## Шаг 5: Сборка десктоп-бинарников

### 5.1. Структура релиза

```
dist/
├── audiobot-panel-linux-amd64/
│   ├── audiobot-panel          (Go бинарник)
│   ├── audiobot-panel-electron (Electron app)
│   └── ...
├── audiobot-panel-linux-arm64/
│   ├── audiobot-panel
│   ├── audiobot-panel-electron
│   └── ...
├── audiobot-panel-windows-amd64/
│   ├── audiobot-panel.exe
│   ├── audiobot-panel-electron.exe
│   └── ...
```

### 5.2. Скрипт сборки: `build.sh`

Создать в корне проекта:

```bash
#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="$PROJECT_ROOT/dist"

echo "=== Building audiobot-panel v$VERSION ==="

# ────────────────────────────────────────────
# 1. Build frontend (Preact + Vite)
# ────────────────────────────────────────────
echo "[1/4] Building frontend..."
cd "$PROJECT_ROOT/frontend"
npm ci
npm run build
echo "  → frontend/build/ ($(du -sh build | cut -f1))"

# ────────────────────────────────────────────
# 2. Build Go backend for each target
# ────────────────────────────────────────────
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

  echo "  → $OS/$ARCH..."
  mkdir -p "$DIST_DIR/audiobot-panel-${OS}-${ARCH}"

  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build \
    -ldflags="-s -w -X main.version=$VERSION" \
    -o "$DIST_DIR/audiobot-panel-${OS}-${ARCH}/audiobot-panel${SUFFIX}" \
    ./backend/
done

# Copy frontend build into each dist directory (Go server serves it)
for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%%/*}"
  ARCH="${TARGET##*/}"
  cp -r "$PROJECT_ROOT/frontend/build" \
    "$DIST_DIR/audiobot-panel-${OS}-${ARCH}/frontend-build"
done

# ────────────────────────────────────────────
# 3. Package Electron apps
# ────────────────────────────────────────────
echo "[3/4] Packaging Electron apps..."
cd "$PROJECT_ROOT/electron"
npm ci

for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%%/*}"
  ARCH="${TARGET##*/}"
  DIST_NAME="audiobot-panel-${OS}-${ARCH}"
  DIST_PATH="$DIST_DIR/$DIST_NAME"

  echo "  → $OS/$ARCH Electron..."

  # Copy backend binary and frontend into Electron's resource dir
  mkdir -p "$DIST_PATH/resources"
  cp "$DIST_PATH/audiobot-panel"* "$DIST_PATH/resources/" 2>/dev/null || true
  cp -r "$PROJECT_ROOT/frontend/build" "$DIST_PATH/resources/frontend-build" 2>/dev/null || true

  # Electron pack using electron-packager or electron-builder
  # Using electron-packager for simplicity:
  npx electron-packager . \
    --platform="$OS" \
    --arch="$ARCH" \
    --out="$DIST_PATH/electron-app" \
    --overwrite \
    --asar \
    --app-version="$VERSION" \
    --name="AudioBot Panel" \
    --executable-name="audiobot-panel-electron" \
    || echo "  ⚠ electron-packager failed for $OS/$ARCH (may need electron-builder)"
done

# ────────────────────────────────────────────
# 4. Create archives
# ────────────────────────────────────────────
echo "[4/4] Creating archives..."
cd "$DIST_DIR"

for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%%/*}"
  ARCH="${TARGET##*/}"
  DIST_NAME="audiobot-panel-${OS}-${ARCH}"

  if [ "$OS" = "windows" ]; then
    zip -r "${DIST_NAME}.zip" "$DIST_NAME"
    echo "  → ${DIST_NAME}.zip"
  else
    tar czf "${DIST_NAME}.tar.gz" "$DIST_NAME"
    echo "  → ${DIST_NAME}.tar.gz"
  fi
done

echo ""
echo "=== Build complete ==="
echo "Artifacts in $DIST_DIR/:"
ls -lh "$DIST_DIR"/*.{tar.gz,zip} 2>/dev/null || echo "(no archives yet)"
```

### 5.3. Альтернатива: electron-builder (рекомендуется)

`electron-packager` прост, но `electron-builder` лучше для релизов:

```bash
npm install --save-dev electron-builder
```

`electron/package.json` расширить:

```json
{
  "build": {
    "appId": "com.audiobot.panel",
    "productName": "AudioBot Panel",
    "directories": {
      "output": "../dist"
    },
    "files": [
      "main.js",
      "preload.js",
      "package.json"
    ],
    "extraResources": [
      {
        "from": "../backend/audiobot-panel",
        "to": "backend/audiobot-panel"
      },
      {
        "from": "../frontend/build",
        "to": "frontend/build"
      }
    ],
    "linux": {
      "target": [
        { "target": "AppImage", "arch": ["x64", "arm64"] },
        { "target": "tar.gz", "arch": ["x64", "arm64"] }
      ],
      "category": "Utility"
    },
    "win": {
      "target": [
        { "target": "portable", "arch": ["x64"] },
        { "target": "nsis", "arch": ["x64"] }
      ]
    }
  }
}
```

Сборка:

```bash
# Linux amd64
electron-builder --linux --x64

# Linux arm64
electron-builder --linux --arm64

# Windows amd64
electron-builder --win --x64
```

---

## Шаг 6: Кросс-компиляция Go под ARM

Go поддерживает кросс-компиляцию из коробки:

```bash
# На любой машине (в т.ч. Windows/x64):
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o audiobot-panel ./backend/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o audiobot-panel ./backend/
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o audiobot-panel.exe ./backend/
```

`CGO_ENABLED=0` обязателен для статической линковки — бинарник работает без libc на ARM.

---

## Шаг 7: GitHub Release через `gh`

### 7.1. Создать тег

```bash
git tag -a v1.0.0 -m "Release v1.0.0: Preact+Vite migration"
git push origin v1.0.0
```

### 7.2. Создать релиз и загрузить бинарники

```bash
gh release create v1.0.0 \
  --title "v1.0.0 — Preact+Vite migration" \
  --notes "## Изменения
- Фронтенд переписан с React+CRA на Preact+Vite
- JS бандл: 40 KB → 4 KB (gzip)
- node_modules: 289 MB → 30 MB
- Сборка на ARM: 60 сек → 5 сек
- Тёмная тема по DESIGN.md (Notion-style)
- VP8 keepalive боты вместо Opus (фикс туннеля)
- Pause/resume API для координации с olcrtc

## Платформы
- Linux (amd64): AppImage + tar.gz
- Linux (arm64): AppImage + tar.gz
- Windows (amd64): portable exe + nsis installer" \
  dist/audiobot-panel-linux-amd64.tar.gz \
  dist/audiobot-panel-linux-arm64.tar.gz \
  dist/audiobot-panel-windows-amd64.zip
```

### 7.3. Альтернатива: GitHub Actions для автоматической сборки

Если нужен CI/CD, создать `.github/workflows/release.yml`:

```yaml
name: Build & Release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  build:
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
            suffix: ""
            archive: tar.gz
          - os: ubuntu-latest
            goos: linux
            goarch: arm64
            suffix: ""
            archive: tar.gz
          - os: windows-latest
            goos: windows
            goarch: amd64
            suffix: .exe
            archive: zip

    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"

      - uses: actions/setup-node@v4
        with:
          node-version: "22"

      - name: Build frontend
        run: |
          cd frontend
          npm ci
          npm run build

      - name: Build Go backend
        env:
          CGO_ENABLED: 0
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: |
          go build -ldflags="-s -w" -o audiobot-panel${{ matrix.suffix }} ./backend/

      - name: Package
        run: |
          mkdir -p dist/audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}
          cp audiobot-panel${{ matrix.suffix }} dist/audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}/
          cp -r frontend/build dist/audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}/frontend-build
          cd dist
          if [ "${{ matrix.archive }}" = "zip" ]; then
            zip -r audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}.zip audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}
          else
            tar czf audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}
          fi
        shell: bash

      - uses: actions/upload-artifact@v4
        with:
          name: audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}
          path: dist/audiobot-panel-${{ matrix.goos }}-${{ matrix.goarch }}.*

  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4

      - name: Create GitHub Release
        run: |
          gh release create ${{ github.ref_name }} \
            --title "${{ github.ref_name }}" \
            --notes "Automated build" \
            audiobot-panel-*/*
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## Порядок выполнения

```
1. Удалить CRA: rm -rf frontend/node_modules frontend/package-lock.json frontend/build
   └─ verify: директория frontend/ содержит только src/ public/ compress-assets.js

2. Создать новый frontend/package.json + vite.config.ts + tsconfig.json + index.html
   └─ verify: npm install проходит, npm run build создаёт build/

3. Переписать src/index.tsx (render from preact)
   └─ verify: npm run dev показывает страницу

4. Механическая замена импортов во всех .tsx:
   "react" → "preact/hooks", "react-dom" → "preact"
   └─ verify: npm run build без ошибок

5. Проверить UI по DESIGN.md (тёмная тема):
   - Кнопки: 8px radius, прямоугольные (не pill)
   - Карточки: 12px radius
   - Инпуты: 44px height
   - Primary: #7c3aed (purple)
   - Шрифт: Inter
   └─ verify: визуальная проверка в браузере

6. Создать build.sh в корне проекта
   └─ verify: bash build.sh — все бинарники собираются

7. Создать релиз:
   git tag -a v1.0.0 -m "Preact+Vite migration"
   git push origin v1.0.0
   bash build.sh v1.0.0
   gh release create v1.0.0 dist/*.tar.gz dist/*.zip
   └─ verify: github.com/USER/infinity-room-panel/releases содержит бинарники
```

---

## Ожидаемый результат

| Метрика | До (React+CRA) | После (Preact+Vite) |
|---------|---------------|---------------------|
| node_modules | 289 MB | ~30 MB |
| JS бандл (gzip) | ~40 KB | ~4 KB |
| CSS | 8 KB | 8 KB (без изменений) |
| RAM при сборке | 500+ MB | ~50 MB |
| Время сборки на ARM | 30-60 сек | 2-5 сек |
| Зависимости рантайма | react + react-dom (2) | preact (1) |
| Фреймворк-оверhead | ~40 KB gzip | ~3 KB gzip |

---

## Примечание: Electron не трогается

Electron — обёртка, которая:
1. Спавнит Go-бинарник (бэкенд)
2. Открывает BrowserWindow на `http://localhost:PORT`
3. Бэкенд раздаёт `frontend/build/` как статику

Замена React→Preact **не влияет** на Electron. Бэкенд всё так же раздаёт статику из `frontend/build/`. Electron всё так же открывает URL. Единственное изменение — содержимое `frontend/build/` становится легче.
