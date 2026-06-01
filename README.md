# AudioBot Panel

> ### ОШИБКА: НЕ ИДЕТ ТРАФИК.
> - Требуются исправления со стороны olcrtc. 
> - Исправлено в [Olcrtc_manager](https://github.com/Oleglog/Olcrtc_manager) для WB stream vp8channel
> - Панель не совместима с [openlibrecommunity/olcrtc](https://github.com/openlibrecommunity/olcrtc)

Веб-панель для управления аудио-ботами в видеоконференциях Jitsi, Яндекс Телемост и WB Stream. Боты подключаются к комнате как участники и воспроизводят загруженное аудио (MP3 → Opus) через WebRTC.

## Архитектура

```
infinity-room-panel/
├── backend/           # Go — HTTP API, бот-менеджер, WebRTC-стриминг
│   ├── api/           #   REST + WebSocket эндпоинты
│   ├── auth/          #   JWT-аутентификация
│   ├── bot/           #   Боты для Jitsi / Телемост / WB Stream
│   ├── storage/       #   Хранение загруженных аудиофайлов
│   └── main.go        #   Точка входа
├── frontend/          # React + TypeScript — UI панели
│   └── src/           #   Компоненты, API-клиент, стили
├── electron/          # Electron-обёртка для десктопа
├── config/            # Конфигурация аутентификации (автосоздаётся)
├── data/              #   Загруженные аудиофайлы
└── DEPLOY.md          # Инструкция по развёртыванию
```

## Текущий статус платформ

| Платформа | Подключение | Воспроизведение аудио | Примечание |
|-----------|:-----------:|:--------------------:|------------|
| **WB Stream** | ✅ | ✅ | Полностью рабочий |
| **Jitsi** | ✅ | ❌ | Боты подключаются, видны участникам, но звук не воспроизводится |
| **Телемост** | ✅ | ❌ | Боты подключаются, видны участникам, но звук не воспроизводится |

## Совместимость
Для лучшей совместимости используйте:
- [Olcrtc_manager](https://github.com/Oleglog/Olcrtc_manager) - сервер
- [Exclave_olcrtc](https://github.com/Oleglog/Exclave_olcrtc) - клиент

### Известные проблемы

**Jitsi** — боты не воспроизводят аудио:
- `session-accept` отправляет пустой `<transport>` без ICE-credentials (ufrag, pwd, fingerprint) и кандидатов — Jicofo не может установить WebRTC-соединение
- Бот не отправляет свои ICE candidates обратно через `transport-info` — нет `OnICECandidate` callback
- Парсинг `transport-info` неполный — поддерживается только Harmony-формат

**Телемост** — боты не воспроизводят аудио:
- PlanB SDP семантика — исправлена (SDPSemanticsPlanB), но остаются проблемы с SDP answer и ICE negotiation
- `capabilitiesOffer` была неполной — расширена (добавлены `offerAnswerMode`, `slotsMode`, etc.), но нужна проверка

## Быстрый старт

```bash
# 1. Собрать фронтенд
cd frontend && npm install && npm run build && cd ..

# 2. Собрать бэкенд
go build -o backend/audiobot-panel.exe ./backend/

# 3. Запустить
./backend/audiobot-panel
```

Полная инструкция по развёртыванию (Windows, Linux, Docker, Electron): **[DEPLOY.md](DEPLOY.md)**

## Возможности

- **Мульти-комната** — запуск ботов одновременно в нескольких комнатах и сервисах
- **Загрузка аудио** — MP3 файлы конвертируются в Opus через ffmpeg
- **Управление** — старт/стоп каждой комнаты независимо
- **Live-статус** — WebSocket обновления статуса ботов в реальном времени
- **Тёмная тема** — Notion-style дизайн
- **Electron** — десктоп-приложение без пароля (автологин)
