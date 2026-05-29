# Инструкция по развертыванию AudioBot Panel

## Содержание

- [Требования](#требования)
- [Развертывание на Windows](#развертывание-на-windows)
- [Развертывание на Linux](#развертывание-на-linux)
- [Запуск веб-версии](#запуск-веб-версии)
- [Запуск Electron-приложения](#запуск-electron-приложения)
- [Установка ffmpeg](#установка-ffmpeg)
- [Конфигурация](#конфигурация)
- [API-эндпоинты](#api-эндпоинты)
- [Устранение неполадок](#устранение-неполадок)

---

## Требования

| Компонент | Версия | Назначение |
|---|---|---|
| Go | 1.22+ | Сборка бэкенда |
| Node.js | 20+ | Сборка фронтенда |
| npm | 10+ | Управление зависимостями фронтенда |
| ffmpeg | любой | Декодирование MP3 → Opus |
| Git | любой | Клонирование репозитория |

---

## Развертывание на Windows

### 1. Установите зависимости

**Go:**
```powershell
# Через winget
winget install GoLang.Go

# Или скачайте с https://go.dev/dl/
# Проверка:
go version
```

**Node.js:**
```powershell
# Через winget
winget install OpenJS.NodeJS.LTS

# Или скачайте с https://nodejs.org/
# Проверка:
node --version
npm --version
```

**ffmpeg:**
```powershell
# Через winget (рекомендуется)
winget install Gyan.FFmpeg

# Через Chocolatey
choco install ffmpeg

# Вручную:
# 1. Скачайте с https://www.gyan.dev/ffmpeg/builds/ → ffmpeg-release-essentials.zip
# 2. Распакуйте, например, в C:\ffmpeg
# 3. Добавьте C:\ffmpeg\bin в PATH:
#    - Win → "environment" → "Edit the system environment variables"
#    - "Environment Variables" → System variables → Path → Edit → New → C:\ffmpeg\bin
# 4. Перезапустите терминал

# Проверка:
ffmpeg -version
```

### 2. Клонируйте репозиторий

```powershell
cd C:\Users\<User>\Documents
git clone <repo-url> infinity-room-panel
cd infinity-room-panel
```

### 3. Соберите фронтенд

```powershell
cd frontend
npm install --legacy-peer-deps
npm run build
cd ..
```

### 4. Соберите бэкенд

```powershell
go mod tidy
go build -o backend/audiobot-panel.exe ./backend/
```

### 5. Запустите

См. разделы [Запуск веб-версии](#запуск-веб-версии) или [Запуск Electron-приложения](#запуск-electron-приложения).

---

## Развертывание на Linux

### 1. Установите зависимости

**Go:**
```bash
# Через snap
sudo snap install go --classic

# Или через apt (Ubuntu 22.04+)
sudo apt update
sudo apt install golang-go

# Или скачайте с https://go.dev/dl/
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Проверка:
go version
```

**Node.js:**
```bash
# Через NodeSource (Ubuntu/Debian)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Или через nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
nvm install 20

# Проверка:
node --version
npm --version
```

**ffmpeg:**
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install ffmpeg

# CentOS/RHEL
sudo yum install epel-release
sudo yum install ffmpeg

# Arch
sudo pacman -S ffmpeg

# Проверка:
ffmpeg -version
```

### 2. Клонируйте репозиторий

```bash
cd ~
git clone <repo-url> infinity-room-panel
cd infinity-room-panel
```

### 3. Соберите фронтенд

```bash
cd frontend
npm install --legacy-peer-deps
npm run build
cd ..
```

### 4. Соберите бэкенд

```bash
go mod tidy
go build -o backend/audiobot-panel ./backend/
```

### 5. Запустите

См. раздел [Запуск веб-версии](#запуск-веб-версии) ниже.

---

## Запуск веб-версии

### Прямой запуск (разработка / тестирование)

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

### Настройка порта

По умолчанию сервер запускается на порту `8080`. Изменить порт:

```bash
# Linux
PORT=3000 ./backend/audiobot-panel

# Windows (PowerShell)
$env:PORT="3000"; .\backend\audiobot-panel.exe
```

### Откройте панель

Перейдите в браузере: `http://localhost:8080` (или выбранный порт).

Введите пароль, полученный при первом запуске.

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
ExecStart=/opt/audiobot-panel/backend/audiobot-panel
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

### Запуск через Docker (опционально)

```dockerfile
FROM golang:1.22-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o backend/audiobot-panel ./backend/

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/backend/audiobot-panel ./backend/
COPY --from=builder /app/frontend/build ./frontend/build/
COPY --from=builder /app/data ./data/
EXPOSE 8080
CMD ["./backend/audiobot-panel"]
```

```bash
docker build -t audiobot-panel .
docker run -p 8080:8080 -v audiobot-data:/app/data -v audiobot-config:/app/config audiobot-panel
# Пароль будет в логах контейнера:
docker logs <container-id>
```

---

## Запуск Electron-приложения

> Только для Windows/macOS/Linux (десктоп).

### Предварительная сборка

Убедитесь, что бэкенд уже скомпилирован (см. шаг 4 выше).

### Установка и запуск

```bash
cd electron
npm install
npm start
```

При запуске Electron:
1. Автоматически выбирается свободный порт
2. Запускается Go-бэкенд с `ELECTRON_MODE=1`
3. Открывается окно браузера с UI
4. **Пароль не требуется** — аутентификация отключена

При закрытии окна — бэкенд-процесс автоматически завершается.

---

## Установка ffmpeg

ffmpeg необходим для конвертации MP3 → Opus. Без него загрузка и воспроизведение аудио работать не будет.

### Windows

```powershell
# Способ 1: winget (рекомендуется)
winget install Gyan.FFmpeg

# Способ 2: Chocolatey
choco install ffmpeg

# Способ 3: Вручную
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
# Удалите файл конфигурации — при следующем запуске будет сгенерирован новый пароль
rm config/auth.json       # Linux
del config\auth.json       # Windows
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
| `POST` | `/api/session/start` | Запустить сессию ботов |
| `POST` | `/api/session/stop` | Остановить сессию |
| `GET` | `/api/session/status` | WebSocket: live-статус ботов |

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
  -d '{"service":"jitsi","room_input":"https://meet.cryptopro.ru/e4r56y","bot_count":2,"file_id":"<id>","loop":true}' \
  http://localhost:8080/api/session/start
```

---

## Устранение неполадок

### «404 page not found» при открытии панели

Бэкенд не находит директорию `frontend/build`. Решения:

1. Убедитесь, что фронтенд собран: `cd frontend && npm run build`
2. Запускайте бинарник из корня проекта: `cd infinity-room-panel && ./backend/audiobot-panel`
3. Или запускайте через `go run`: `go run ./backend/`

### Electron запрашивает пароль

Бэкенд должен запускаться с переменной `ELECTRON_MODE=1`. Проверьте:
- `electron/main.js` передаёт `ELECTRON_MODE: '1'` в `env` при spawn
- Go-бинарник запускается с `cwd: projectRoot` (корень проекта)
- Пересоберите фронтенд после обновления кода: `cd frontend && npm run build`

### «ffmpeg not found» / аудио не загружается

Убедитесь, что ffmpeg установлен и доступен в PATH:

```bash
ffmpeg -version
# Если команда не найдена — установите ffmpeg (см. раздел выше)
```

### Ошибка «invalid token» (веб-версия)

JWT-токен истекает через 24 часа. Получите новый:
1. Откройте панель в браузере
2. Введите пароль заново

### Сброс всего

```bash
# Удалить конфиг и данные
rm -rf config/ data/audio/    # Linux
rmdir /s config data\audio    # Windows

# При следующем запуске будет сгенерирован новый пароль
```

### Порт занят

```bash
# Указать другой порт
PORT=3000 ./backend/audiobot-panel    # Linux
$env:PORT="3000"; .\backend\audiobot-panel.exe   # Windows PowerShell
```
