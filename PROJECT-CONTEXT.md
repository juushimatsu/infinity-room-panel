# Контекст проекта: infinity-room-panel

## Что это

**AudioBot Panel** — веб-панель для управления аудио-ботами в видеоконференциях. Боты подключаются к комнатам (Jitsi, Telemost, WB Stream) и воспроизводят Opus-аудио, чтобы держать комнаты постоянно активными. Панель доступна как Electron-приложение (десктоп) и как headless-сервер (доступ через браузер).

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                      Пользователь                            │
│                         │                                   │
│    ┌────────────────────┴────────────────────┐              │
│    │              Electron App                │              │
│    │  (main.js → spawn Go backend → Window) │              │
│    └────────────────────┬────────────────────┘              │
│                         │                                   │
│    ┌────────────────────┴────────────────────┐              │
│    │         Preact Frontend (Vite)           │              │
│    │  ┌─────────┐ ┌──────────┐ ┌───────────┐ │              │
│    │  │LoginPage│ │NewRoom   │ │RoomCards  │ │              │
│    │  │         │ │Form      │ │(status)   │ │              │
│    │  └─────────┘ └──────────┘ └───────────┘ │              │
│    └────────────────────┬────────────────────┘              │
│                         │ HTTP/WebSocket                    │
│    ┌────────────────────┴────────────────────┐              │
│    │           Go Backend (gorilla/mux)      │              │
│    │  ┌────────┐ ┌────────┐ ┌──────────────┐ │              │
│    │  │Auth    │ │Audio   │ │Room          │ │              │
│    │  │(JWT)   │ │(upload)│ │(start/stop)  │ │              │
│    │  └────────┘ └────────┘ └──────────────┘ │              │
│    │           ↓ BotManager                 │              │
│    │  ┌──────────────────────────────────┐   │              │
│    │  │  goroutine per bot               │   │              │
│    │  │  RunJitsiBot / RunTelemostBot    │   │              │
│    │  │  RunWBStreamBot (LiveKit)        │   │              │
│    │  └──────────────────────────────────┘   │              │
│    └─────────────────────────────────────────┘              │
│                         │                                   │
│    ┌────────────────────┴────────────────────┐              │
│    │      LiveKit / Jitsi / Telemost SFU      │              │
│    │         (WebRTC видеоконференция)          │              │
│    └───────────────────────────────────────────┘              │
└─────────────────────────────────────────────────────────────┘
```

## Компоненты

### 1. Frontend (`frontend/`)

| Файл | Назначение |
|------|-----------|
| `src/App.tsx` | Корневой компонент: роутинг между LoginPage и основным UI |
| `src/api/client.ts` | HTTP-клиент: upload/list audio, start/stop room, login, WebSocket |
| `src/components/LoginPage.tsx` | Форма входа (в веб-режиме) |
| `src/components/NewRoomForm.tsx` | Форма создания комнаты: выбор сервиса, URL, кол-во ботов, аудио |
| `src/components/RoomCard.tsx` | Карточка комнаты со статусом ботов, кнопками: запуск, остановка, перезапуск, редактирование, удаление |
| `src/components/EditRoomForm.tsx` | Форма редактирования настроек комнаты (URL, сервис, боты, аудио) |
| `src/components/WBAccountSettings.tsx` | Настройки WB Stream аккаунта для антиотключения рум (JSON dump, интервал, имя бота) |
| `src/components/AudioUpload.tsx` | Drag-and-drop загрузка MP3 |
| `src/styles.css` | CSS-переменные (тёмная тема, Notion-style) |

**Стек**: Preact 10 + Vite 6 + TypeScript. Был React 18 + CRA.

**Сборка**: `npm run build` → `frontend/build/` (статика, раздаётся Go-сервером).

### 2. Backend (`backend/`)

| Файл | Назначение |
|------|-----------|
| `main.go` | Точка входа: инициализация auth, storage, bot manager, HTTP-сервер на gorilla/mux, SPA-фоллбэк |
| `api/handlers.go` | HTTP-хендлеры: `/api/audio/*`, `/api/room/*`, `/api/auth/*` |
| `api/ws.go` | WebSocket `/api/room/status` — live-статус ботов |
| `bot/manager.go` | `BotManager`: запуск/остановка комнат, управление goroutine ботов, status pub/sub, интеграция с RoomConfigStore |
| `bot/wbstream.go` | `RunWBStreamBot`: подключение к WB Stream через LiveKit SDK, публикация Opus-аудио |
| `bot/wbstream_account.go` | `RunWBAccountKeeper`: фоновый циклический заход под авторизованным аккаунтом (без аудио) для предотвращения автоотключения рум |
| `storage/room_config.go` | `RoomConfigStore`: персистентное хранение конфигураций комнат в `data/room_configs.json` |
| `config/wb_account.go` | `AccountStore`: хранение настроек WB аккаунта в `data/wb_account.json` |
| `bot/jitsi.go` | `RunJitsiBot`: подключение к Jitsi через Pion WebRTC, Opus аудио |
| `bot/telemost.go` | `RunTelemostBot`: подключение к Telemost (Goolom), PlanB SDP, Opus аудио |
| `bot/audio.go` | `DecodeMP3ToPCM` → `EncodePCMToOpusFrames` через ffmpeg |
| `auth/auth.go` | JWT-аутентификация, bcrypt-хеш пароля, генерация секрета |
| `storage/storage.go` | Хранение загруженных MP3 в `data/audio/` |

**Стек**: Go 1.25, gorilla/mux, LiveKit server-sdk-go/v2 (v2.16.2), Pion webrtc/v4.

### 3. Electron (`electron/`)

| Файл | Назначение |
|------|-----------|
| `main.js` | Главный процесс: выбор свободного порта, spawn Go-бэкенда, BrowserWindow, cleanup |
| `preload.js` | `contextBridge`: пробрасывает `window.electronAPI.isElectron = true` |

**Ключевые настройки для ARM**: `app.disableHardwareAcceleration()`, отключены неиспользуемые Chromium-фичи.

### 4. OlcRTC (отдельный проект в `temp/`)

**Exclave_olcrtc** (клиент) + **Olcrtc_manager** (сервер) — зашифрованный TCP-over-WebRTC туннель.

| Компонент | Назначение |
|-----------|-----------|
| `internal/engine/livekit/livekit.go` | LiveKit engine: `AutoSubscribe=true`, `OnTrackSubscribed` фильтрует только VP8 |
| `internal/transport/vp8channel/transport.go` | VP8-транспорт: epoch-заголовки, bindingToken (FNV1a от RoomURL), single-peer latch |
| `internal/transport/vp8channel/kcpconn.go` | KCP поверх VP8-фреймов |
| `internal/client/client.go` | Клиентский smux + SOCKS5 (127.0.0.1:8808) |
| `internal/server/server.go` | Серверный smux + handshake |
| `internal/control/` | Liveness ping/pong, reconnect по timeout |

**Архитектура туннеля**:
```
Приложение → SOCKS5 → olcrtc client → WebRTC/LiveKit → olcrtc server → интернет
                ↑                           ↓
            smux stream               VP8 video frames
            XChaCha20-Poly1305        KCP-packets
            vp8channel                LiveKit SFU
```

## Потоки данных

### Запуск ботов (WB Stream)

```
1. Пользователь → POST /api/room/start {service: "wbstream", room_input, bot_count, file_id}
2. BotManager.StartRoom() → создаёт Room, для каждого бота:
   a. Загружает Opus-фреймы из data/audio/<file_id>.mp3 (через ffmpeg)
   b. Запускает goroutine: RunWBStreamBot()
3. RunWBStreamBot:
   a. WB Stream API: guest-register → join-room → connection-details → LiveKit token
   b. lksdk.NewRoom() → JoinWithToken(serverUrl, roomToken, WithAutoSubscribe(false))
   c. webrtc.NewTrackLocalStaticSample(Opus) → PublishTrack()
   d. Ticker 20ms → WriteSample(opusFrame) в LiveKit SFU
4. WebSocket /api/room/status → live-статус каждого бота (connecting/active/error/stopped)
```

### Поток аудио

```
MP3 файл → ffmpeg decode → PCM int16 48kHz mono → ffmpeg opus encode 24kbps 20ms frames
    → [][]byte (Opus frames) → BotManager → goroutine → WriteSample every 20ms → LiveKit SFU
```

### Взаимодействие с olcrtc (конфликт)

```
Боты в WB Stream комнате:
  - Публикуют Opus-аудио (1-3 бота)
  - WithAutoSubscribe(false) — не подписываются на чужие треки

Olcrtc server подключается к той же комнате:
  - WithAutoSubscribe(true) — автоматически подписывается на ВСЕ треки
  - Включая Opus-аудио ботов
  - SFU форвардит RTP-пакеты аудио на olcrtc server
  - PeerConnection тратит ресурсы на audio transceiver
  - SFU может throttling'ить VP8-пакеты туннеля
  → KCP теряет пакеты → liveness timeout → reconnect → цикл
```

## Ключевые API-эндпоинты

| Метод | Путь | Аутентификация | Описание |
|-------|------|---------------|----------|
| `GET` | `/api/auth/mode` | Нет | `{electron: true/false}` |
| `POST` | `/api/auth/login` | Нет | `{password}` → `{token}` |
| `GET` | `/api/auth/check` | Нет | `{valid: true/false}` |
| `POST` | `/api/audio/upload` | Bearer | Загрузка MP3 (multipart) |
| `GET` | `/api/audio/list` | Bearer | Список аудиофайлов |
| `POST` | `/api/room/start` | Bearer | Запуск комнаты с ботами |
| `POST` | `/api/room/stop` | Bearer | Остановка ботов (комната остаётся в списке, неактивная) |
| `POST` | `/api/room/delete` | Bearer | Удалить комнату из панели (остановка + удаление конфига) |
| `POST` | `/api/room/restart` | Bearer | Перезапустить комнату |
| `POST` | `/api/room/update` | Bearer | Изменить настройки комнаты (URL, сервис, боты, аудио) |
| `POST` | `/api/room/start-from-config` | Bearer | Запустить остановленную комнату из сохранённого конфига |
| `POST` | `/api/room/pause` | Bearer | Пауза ботов в комнате |
| `POST` | `/api/room/resume` | Bearer | Возобновление ботов |
| `GET` | `/api/room/list` | Bearer | Список всех комнат (активных и неактивных) |
| `GET` | `/api/room/status` | Bearer | WebSocket live-статус |
| `GET` | `/api/wbstream/account` | Bearer | Получить настройки WB аккаунта |
| `POST` | `/api/wbstream/account` | Bearer | Сохранить настройки WB аккаунта |
| `POST` | `/api/wbstream/account/stop` | Bearer | Немедленно остановить keeper-бота |

## Файловая структура

```
infinity-room-panel/
├── backend/
│   ├── main.go              # Точка входа: инициализация auth, storage, room config, account store, bot manager
│   ├── api/
│   │   ├── handlers.go      # HTTP-хендлеры (room CRUD, WB account, audio, auth)
│   │   └── ws.go            # WebSocket
│   ├── bot/
│   │   ├── manager.go       # Управление ботами + RoomConfigStore + WBAccountKeeper
│   │   ├── wbstream.go      # WB Stream бот (LiveKit)
│   │   ├── wbstream_account.go # WB Account keeper (антиотключение)
│   │   ├── jitsi.go         # Jitsi бот
│   │   ├── telemost.go      # Telemost бот
│   │   └── audio.go         # MP3 → Opus через ffmpeg
│   ├── auth/
│   │   └── auth.go          # JWT, bcrypt, пароли
│   ├── storage/
│   │   ├── storage.go       # Файловое хранилище MP3
│   │   └── room_config.go   # Хранение конфигураций комнат
│   └── config/
│       └── wb_account.go    # Хранение настроек WB аккаунта
├── frontend/
│   ├── src/
│   │   ├── App.tsx          # Корневой компонент
│   │   ├── api/client.ts   # HTTP-клиент
│   │   ├── components/     # UI-компоненты
│   │   └── styles.css      # Тёмная тема (Notion-style)
│   ├── build/              # Статика (генерируется Vite)
│   └── package.json        # Preact + Vite
├── electron/
│   ├── main.js             # Electron main process
│   ├── preload.js          # Context bridge
│   └── package.json        # Electron 30
├── data/
│   ├── audio/              # Загруженные MP3 + metadata.json
│   ├── room_configs.json   # Конфигурации комнат (сохраняются при создании)
│   └── wb_account.json     # Настройки WB Stream аккаунта
├── scripts/
│   └── wb-extract-cookies.js # Скрипт для извлечения сессии из WB Stream (браузер/приложение)
├── temp/
│   ├── Exclave_olcrtc/     # OlcRTC клиент
│   └── Olcrtc_manager/     # OlcRTC сервер
├── go.mod                  # Go зависимости (LiveKit SDK v2.16.2, pion/webrtc/v4)
├── build.sh                # Скрипт сборки бинарников
└── DEPLOY.md               # Инструкция по развёртыванию
```

## Сборка и релиз

| Шаг | Команда | Результат |
|-----|---------|-----------|
| Фронтенд | `cd frontend && npm ci && npm run build` | `frontend/build/` (~9 KB gzip JS) |
| Бэкенд (dev) | `go build -o backend/audiobot-panel ./backend/` | ~30 MB бинарник |
| Полная сборка | `bash build.sh v1.x.x` | `dist/*.tar.gz` + `dist/*.zip` для linux/amd64, linux/arm64, windows/amd64, windows/386 |
| Релиз | `gh release create v1.x.x dist/*` | GitHub Release с бинарниками |

## Аутентификация

| Режим | Пароль | Механизм |
|-------|--------|----------|
| Electron | Не требуется | `ELECTRON_MODE=1` → аутентификация отключена |
| Headless (веб) | Требуется | `config/auth.json`: bcrypt-хеш + JWT-токен (24ч) |

**Первый запуск headless**: случайный 16-символьный пароль печатается в stdout:
```
=== Сгенерированный пароль для входа: aB3kX9pL2mN7qR4 ===
```

**Сброс пароля**: удалить `config/auth.json` → перезапустить → новый пароль.

## Известные проблемы

| Проблема | Статус | Решение |
|----------|--------|---------|
| Боты мешают olcrtc-туннелю | Активно | Требуется фикс на стороне olcrtc: `AutoSubscribe=false` или `pub.SetSubscribed(false)` для аудио |
| | | Обходной путь: pause/resume API, ручная координация |
| WB Stream отключает комнаты с только гостевыми ботами | Решено | WB Account Keeper заходит под реальным аккаунтом с заданным интервалом |
| Релизные бинарники linux/arm64 не воспроизводят аудио на Orange Pi | Решено | Рекомендуется сборка на самом устройстве (`go build`); добавлено диагностическое логирование |
| Electron 30 не поддерживает Windows 32-bit | Принят | Win32-релиз содержит только headless Go-бинарник |

## Зависимости

| Компонент | Версия | Примечание |
|-----------|--------|------------|
| Go | 1.25+ | Сборка бэкенда |
| Node.js | 20+ | Сборка фронтенда |
| LiveKit SDK | v2.16.2 | Был v2.1.2, обновлен для совместимости с olcrtc (pion v4) |
| Pion WebRTC | v4 | Совместимость с olcrtc |
| ffmpeg | любой | MP3 → Opus, обязателен для работы аудио |
| Electron | 30 | Desktop-обёртка |
