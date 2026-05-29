# Инструкция по развертыванию AudioBot Panel

## Содержание

- [Требования](#требования)
- [Установка из релиза (рекомендуется)](#установка-из-релиза-рекомендуется)
- [Сборка из исходников](#сборка-из-исходников)
- [Запуск](#запуск)
- [Установка ffmpeg](#установка-ffmpeg)
- [Конфигурация](#конфигурация)
- [API-эндпоинты](#api-эндпоинты)
- [Устранение неполадок](#устранение-неполадок)

---

## Требования

| Компонент | Версия | Назначение |
|---|---|---|
| ffmpeg | любой | Декодирование MP3 → Opus |

Дополнительные требования зависят от способа установки:

| Способ | Доп. требования |
|---|---|
| Electron (десктоп) | — (всё включено в архив) |
| Headless (только бэкенд) | — |
| Сборка из исходников | Go 1.25+, Node.js 20+, npm 10+, Git |

---

## Установка из релиза (рекомендуется)

Скачайте архив для вашей платформы со [страницы релизов](https://github.com/juushimatsu/infinity-room-panel/releases).

### Linux amd64

```bash
# Скачайте архив
wget https://github.com/juushimatsu/infinity-room-panel/releases/latest/download/audiobot-panel-linux-amd64.tar.gz

# Распакуйте
tar xzf audiobot-panel-linux-amd64.tar.gz
cd audiobot-panel-linux-amd64

# Запустите Electron (десктоп)
./electron-app/audiobot-panel-electron-linux-x64/audiobot-panel-electron

# Или headless (доступ через браузер http://localhost:8080)
./audiobot-panel
```

### Linux arm64

```bash
wget https://github.com/juushimatsu/infinity-room-panel/releases/latest/download/audiobot-panel-linux-arm64.tar.gz
tar xzf audiobot-panel-linux-arm64.tar.gz
cd audiobot-panel-linux-arm64

# Electron (требует системные библиотеки — см. ниже)
./electron-app/audiobot-panel-electron-linux-arm64/audiobot-panel-electron

# Или headless
./audiobot-panel
```

Для Electron на ARM может потребоваться установка библиотек:

```bash
sudo apt install -y libgtk-3-0 libnotify4 libnss3 libxss1 libxtst6 xdg-utils libatspi2.0-0 libdrm2 libgbm1 libasound2
```

### Windows amd64

Скачайте `audiobot-panel-windows-amd64.zip`, распакуйте.

```
# Electron (десктоп)
electron-app\audiobot-panel-electron-win32-x64\audiobot-panel-electron.exe

# Или headless
audiobot-panel.exe
```

### Windows 32-bit

Скачайте `audiobot-panel-windows-386.zip`, распакуйте.

Только headless-режим (Electron 30+ не поддерживает ia32):

```
audiobot-panel.exe
```

Откройте `http://localhost:8080` в браузере.

---

## Сборка из исходников

### Зависимости

| Компонент | Версия | Установка |
|---|---|---|
| Go | 1.25+ | https://go.dev/dl/ или `winget install GoLang.Go` |
| Node.js | 20+ | https://nodejs.org/ или `winget install OpenJS.NodeJS.LTS` |
| npm | 10+ | Входит в Node.js |
| ffmpeg | любой | См. [раздел ниже](#установка-ffmpeg) |
| Git | любой | https://git-scm.com/ |

### Клонируйте репозиторий

```bash
git clone https://github.com/juushimatsu/infinity-room-panel.git
cd infinity-room-panel
```

### Соберите фронтенд

```bash
cd frontend
npm ci
npm run build
cd ..
```

### Соберите бэкенд

```bash
go mod tidy
go build -o backend/audiobot-panel ./backend/   # Linux
go build -o backend/audiobot-panel.exe ./backend/ # Windows
```

### Соберите всё сразу (все платформы)

```bash
bash build.sh v1.1.0
```

Скрипт соберёт фронтенд, Go-бинарники для linux/amd64, linux/arm64, windows/amd64, windows/386, упакует Electron и создаст архивы в `dist/`.

---

## Запуск

### Electron-приложение (десктоп)

```bash
cd electron
npm install
npm start
```

При запуске Electron:
1. Автоматически выбирается свободный порт
2. Запускается Go-бэкенд с `ELECTRON_MODE=1`
3. Открывается окно с UI
4. **Пароль не требуется** — аутентификация отключена

При закрытии окна бэкенд-процесс автоматически завершается.

> **ARM-устройства**: Electron на ARM требует системные библиотеки (см. выше). При ошибке SUID sandbox выполните:
> ```bash
> sudo chown root "$(find electron/node_modules/electron -name chrome-sandbox | head -1)"
> sudo chmod 4755 "$(find electron/node_modules/electron -name chrome-sandbox | head -1)"
> ```
> Или запустите с `--no-sandbox`: `npx electron . --no-sandbox`

### Headless (доступ через браузер)

```bash
# Linux
./backend/audiobot-panel

# Windows
.\backend\audiobot-panel.exe
```

При **первом запуске** сервер сгенерирует случайный пароль и выведет его в stdout:

```
=== Сгенерированный пароль для входа: aB3kX9pL2mN7qR4 ===
```

**Сохраните этот пароль!** Он потребуется для входа в веб-панель.

Пароль хранится в хешированном виде в `config/auth.json`. При последующих запусках пароль не генерируется повторно.

Откройте `http://localhost:8080` в браузере и введите пароль.

### Настройка порта

```bash
# Linux
PORT=3000 ./backend/audiobot-panel

# Windows (PowerShell)
$env:PORT="3000"; .\backend\audiobot-panel.exe
```

### Запуск как systemd-сервис (Linux)

Создайте файл `/etc/systemd/system/audiobot-panel.service`:

```ini
[Unit]
Description=AudioBot Panel
After=network.target

[Service]
Type=simple
User=audiobot
WorkingDirectory=/opt/audiobot-panel
ExecStart=/opt/audiobot-panel/audiobot-panel
Environment=PORT=8080
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Активируйте и запустите:

```bash
sudo systemctl daemon-reload
sudo systemctl enable audiobot-panel
sudo systemctl start audiobot-panel

# Просмотр логов (здесь будет сгенерированный пароль при первом запуске):
sudo journalctl -u audiobot-panel -f
```

---

## Установка ffmpeg

ffmpeg необходим для конвертации MP3 → Opus. Без него загрузка и воспроизведение аудио работать не будет.

### Windows

```powershell
# Через winget (рекомендуется)
winget install Gyan.FFmpeg

# Через Chocolatey
choco install ffmpeg

# Вручную:
# 1. Скачайте https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip
# 2. Распакуйте в C:\ffmpeg
# 3. Добавьте C:\ffmpeg\bin в системный PATH
# 4. Перезапустите терминал
```

### Linux (Ubuntu/Debian)

```bash
sudo apt update && sudo apt install ffmpeg
```

### Linux (CentOS/RHEL)

```bash
sudo yum install epel-release
sudo yum install ffmpeg
```

### macOS

```bash
brew install ffmpeg
```

### Проверка

```bash
ffmpeg -version
# Должно вывести версию и информацию о сборке
```

---

## Конфигурация

### Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `PORT` | `8080` | Порт HTTP-сервера |
| `ELECTRON_MODE` | (не задана) | Если `1` — отключает аутентификацию |

### Файловая структура данных

```
infinity-room-panel/
├── config/
│   └── auth.json          # Хеш пароля + JWT-секрет (автосоздаётся)
├── data/
│   ├── audio/
│   │   ├── metadata.json  # Метаданные загруженных MP3
│   │   ├── <uuid>.mp3     # Загруженные аудиофайлы
│   │   └── ...
│   └── names/
│       └── names.go       # Генератор имён (встроен в бинарник)
```

### Сброс пароля (веб-версия)

```bash
rm config/auth.json         # Linux
del config\auth.json         # Windows
# При следующем запуске будет сгенерирован новый пароль
```

---

## API-эндпоинты

### Без аутентификации

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/api/auth/mode` | Режим: `{electron: true/false}` |
| `POST` | `/api/auth/login` | Логин: `{password: "..."}` → `{token: "..."}` |
| `GET` | `/api/auth/check` | Проверка токена: `{valid: true/false}` |

### С аутентификацией (Bearer токен, кроме Electron-режима)

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/api/audio/upload` | Загрузить MP3 (multipart form) |
| `GET` | `/api/audio/list` | Список загруженных файлов |
| `POST` | `/api/room/start` | Запустить комнату с ботами |
| `POST` | `/api/room/stop` | Остановить комнату |
| `GET` | `/api/room/list` | Список активных комнат |
| `GET` | `/api/room/status` | WebSocket: live-статус ботов |
| `POST` | `/api/room/pause` | Пауза ботов в комнате (для координации с olcrtc) |
| `POST` | `/api/room/resume` | Возобновление ботов после паузы |

### Примеры запросов

**Загрузка аудио:**
```bash
curl -X POST -H "Authorization: Bearer <token>" \
  -F "file=@audio.mp3" \
  http://localhost:8080/api/audio/upload
```

**Запуск ботов:**
```bash
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"service":"wbstream","room_input":"https://stream.wb.ru/streams/abc123","bot_count":2,"file_id":"<id>","loop":true}' \
  http://localhost:8080/api/room/start
```

**Пауза ботов (перед запуском olcrtc-туннеля):**
```bash
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"room_id":"room_1"}' \
  http://localhost:8080/api/room/pause
```

**Возобновление ботов:**
```bash
curl -X POST -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"room_id":"room_1"}' \
  http://localhost:8080/api/room/resume
```

---

## Устранение неполадок

### «404 page not found» при открытии панели

Бэкенд не находит директорию `frontend/build`. Решения:

1. Убедитесь, что фронтенд собран: `cd frontend && npm run build`
2. Запускайте бинарник из корня проекта: `cd infinity-room-panel && ./backend/audiobot-panel`
3. Или используйте предсобранный архив из релиза

### Electron запрашивает пароль

Бэкенд должен запускаться с переменной `ELECTRON_MODE=1`. Это автоматически делается Electron-обёрткой. Если пароль запрашивается — проверьте, что запускаете через `npm start` в директории `electron/`, а не бэкенд напрямую.

### «ffmpeg not found» / аудио не загружается

Убедитесь, что ffmpeg установлен и доступен в PATH:

```bash
ffmpeg -version
# Если команда не найдена — установите ffmpeg (см. раздел выше)
```

### Ошибка «invalid token» (веб-версия)

JWT-токен истекает через 24 часа. Получите новый: откройте панель в браузере и введите пароль заново.

### Сброс всего

```bash
rm -rf config/ data/audio/     # Linux
rmdir /s config data\audio     # Windows

# При следующем запуске будет сгенерирован новый пароль
```

### Порт занят

```bash
PORT=3000 ./backend/audiobot-panel              # Linux
$env:PORT="3000"; .\backend\audiobot-panel.exe  # Windows PowerShell
```
