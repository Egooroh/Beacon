# Beacon

Самостоятельно размещаемый агрегатор ошибок на Go. Приложения отправляют события по HTTP; Beacon группирует их в инциденты по отпечатку, подавляет шум и отправляет структурированные оповещения в Telegram.

---

## Содержание

- [Возможности](#возможности)
- [Архитектура](#архитектура)
- [Быстрый старт](#быстрый-старт)
- [Справочник API](#справочник-api)
- [Формат события](#формат-события)
- [Алгоритм группировки](#алгоритм-группировки)
- [Конфигурация](#конфигурация)
- [Структура проекта](#структура-проекта)
- [Разработка](#разработка)
- [Стек технологий](#стек-технологий)

---

## Возможности

| Возможность | Описание |
|---|---|
| **Приём событий** | `POST /ingest` принимает JSON-события; аутентификация по токену на проект |
| **Умная группировка** | SHA-256 отпечаток по нормализованным фреймам стека или сообщению; одна и та же логическая ошибка → один инцидент, независимо от runtime-шума |
| **Нормализация сообщений** | Числа, UUID, hex-адреса, строки в кавычках, email и URL заменяются на стабильные плейсхолдеры до хеширования |
| **Жизненный цикл инцидента** | Четыре статуса: `open` · `resolved` · `muted` · `ignored`; обнаружение регрессии на закрытых инцидентах |
| **Оповещения** | Мгновенное уведомление в Telegram при новом инциденте или регрессии; настраиваемый cooldown защищает от спама |
| **Обнаружение всплесков** | Фоновый воркер сравнивает текущий и предыдущий час; оповещает при превышении порога |
| **Дайджест** | Периодическая сводка топ-N инцидентов для всех подписчиков проекта |
| **Управление инцидентами** | Постраничный список с фильтром по статусу; `PATCH` для смены статуса |
| **Метрики Prometheus** | Доступны на `/metrics` |
| **Плавное завершение** | Graceful shutdown с настраиваемым таймаутом |

---

## Архитектура

Beacon строится по **Clean Architecture** со строгим правилом направления зависимостей:

```
transport/http  ─→  usecase  ─→  domain
adapter         ─→  usecase  ─→  domain
cmd/beacon (точка сборки)
```

`internal/domain` не импортирует ничего из проекта и никаких фреймворков. Все внешние зависимости в use case выражены через интерфейсы, объявленные в пакете-потребителе (порты). `cmd/beacon/main.go` — единственное место, где конкретные типы собираются вместе; DI-фреймворк не используется.

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP-клиенты                            │
└───────────────────────────┬─────────────────────────────────┘
                            │ JSON over HTTP
┌───────────────────────────▼─────────────────────────────────┐
│            transport/http  (chi роутер + middleware)          │
│   /ingest · /projects · /issues · /subscriptions · /metrics  │
└──────┬──────────────┬───────────────┬────────────────────────┘
       │              │               │
┌──────▼──────┐ ┌─────▼──────┐ ┌─────▼────────────────────────┐
│  ingest UC  │ │  issue UC  │ │  subscription UC              │
└──────┬──────┘ └─────┬──────┘ └─────┬────────────────────────┘
       │              │               │
┌──────▼──────────────▼───────────────▼────────────────────────┐
│                        domain                                  │
│   Event · Issue · Project · Subscription · Alert · Fingerprint│
└───────────────────────────────────────────────────────────────┘
       ▲              ▲               ▲
┌──────┴──────┐ ┌─────┴──────┐ ┌─────┴────────────────────────┐
│ grouping UC │ │ alerting UC│ │  digest UC                    │
│ (процессор) │ │ (cooldown) │ │  (периодический отчёт)       │
└──────┬──────┘ └─────┬──────┘ └─────┬────────────────────────┘
       │              │               │
┌──────▼──────────────▼───────────────▼────────────────────────┐
│           adapter/repository/postgres (pgstore)                │
│           adapter/fingerprint                                  │
│           adapter/notify/telegram                              │
└───────────────────────────────┬───────────────────────────────┘
                                │
                        PostgreSQL 16
```

### Фоновые воркеры

Три тикер-горутины работают независимо от HTTP-сервера:

| Воркер | Интервал | Что делает |
|---|---|---|
| `processor` | `BEACON_PROCESS_INTERVAL` (2с) | Читает необработанные события, вычисляет отпечатки, делает upsert инцидентов, запускает оповещения |
| `spike-checker` | `BEACON_DIGEST_INTERVAL` (1ч) | Сравнивает частоту событий за текущий и предыдущий час; оповещает при всплеске |
| `digest` | `BEACON_DIGEST_INTERVAL` (1ч) | Отправляет сводку топ-N инцидентов всем подписчикам |

---

## Быстрый старт

### Docker (рекомендуется)

```bash
# Клонировать репозиторий
git clone https://github.com/Egooroh/beacon.git
cd beacon

# Скопировать файл переменных окружения
cp .env.example .env

# Опционально: добавить токен Telegram-бота для уведомлений
# BEACON_TELEGRAM_TOKEN=<токен от @BotFather>

# Запустить Postgres + Beacon
make docker-up

# Проверить
curl http://localhost:8080/healthz
# {"status":"ok"}
```

### Попробовать за 60 секунд

```bash
# 1. Создать проект — получить одноразовый ingest-токен
curl -s -X POST http://localhost:8080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"my-app"}' | tee project.json

TOKEN=$(cat project.json | python3 -c "import sys,json; print(json.load(sys.stdin)['ingest_token'])")
PROJECT_ID=$(cat project.json | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

# 2. Отправить несколько ошибок (разные user ID → один отпечаток)
for ID in 42 9981 100500; do
  curl -s -X POST http://localhost:8080/api/v1/ingest \
    -H "X-Beacon-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"level\":\"error\",\"message\":\"user $ID not found\"}"
done

# 3. Подождать воркер группировки (≤ 2 с), получить список инцидентов
sleep 3
curl -s "http://localhost:8080/api/v1/projects/$PROJECT_ID/issues" | python3 -m json.tool
# → 1 инцидент, events_count: 3

# 4. Закрыть инцидент
ISSUE_ID=$(curl -s "http://localhost:8080/api/v1/projects/$PROJECT_ID/issues" | \
  python3 -c "import sys,json; print(json.load(sys.stdin)['items'][0]['id'])")

curl -s -X PATCH "http://localhost:8080/api/v1/issues/$ISSUE_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"resolved"}'
```

---

## Справочник API

Все эндпоинты находятся под `/api/v1`. Формат: JSON на входе и выходе. Ошибки возвращаются в виде `{"error":"..."}`.

### Проекты

#### Создать проект
```
POST /api/v1/projects
```

```json
// Запрос
{ "name": "my-service" }

// Ответ 201
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "my-service",
  "ingest_token": "abc123..."   // сохранить в секрете — показывается один раз
}
```

---

### Приём событий

#### Отправить событие об ошибке
```
POST /api/v1/ingest
X-Beacon-Token: <ingest_token>
```

Формат тела запроса описан в разделе [Формат события](#формат-события).

Ответ: `204 No Content`

---

### Инциденты

#### Получить список инцидентов
```
GET /api/v1/projects/{project_id}/issues
```

| Параметр | Тип | По умолчанию | Описание |
|---|---|---|---|
| `status` | string | (все) | Фильтр: `open` · `resolved` · `muted` · `ignored` |
| `limit` | int | 20 | Размер страницы, максимум 100 |
| `offset` | int | 0 | Смещение для пагинации |

```json
// Ответ 200
{
  "items": [
    {
      "id": "...",
      "fingerprint": "a3f9...",
      "title": "user N not found",
      "level": "error",
      "status": "open",
      "events_count": 3,
      "first_seen_at": "2024-01-15T10:00:00Z",
      "last_seen_at":  "2024-01-15T10:05:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

#### Изменить статус инцидента
```
PATCH /api/v1/issues/{issue_id}/status
```

```json
// Запрос
{ "status": "resolved" }   // open | resolved | muted | ignored
```

Ответ: `204 No Content`

Семантика статусов:

| Статус | Поведение |
|---|---|
| `open` | По умолчанию; новые события вызывают немедленные оповещения |
| `resolved` | Закрыт; новое событие вызывает оповещение о **регрессии** и автоматически переоткрывает инцидент |
| `muted` | События продолжают считаться; все оповещения подавлены |
| `ignored` | Без оповещений, без обновления счётчиков |

---

### Подписки

#### Добавить получателя уведомлений
```
POST /api/v1/projects/{project_id}/subscriptions
```

```json
// Запрос
{ "platform": "telegram", "chat_id": "-1001234567890" }

// Ответ 201
{
  "id": "...",
  "project_id": "...",
  "platform": "telegram",
  "chat_id": "-1001234567890"
}
```

Идемпотентный запрос — повторная отправка тех же `(project, platform, chat_id)` возвращает существующую запись.

#### Получить список подписок
```
GET /api/v1/projects/{project_id}/subscriptions
```

---

### Служебные эндпоинты

| Эндпоинт | Описание |
|---|---|
| `GET /healthz` | Liveness — возвращает `{"status":"ok"}` |
| `GET /readyz` | Readiness — проверяет соединение с базой данных |
| `GET /metrics` | Метрики Prometheus |

---

## Формат события

```json
{
  "level": "error",
  "message": "failed to process payment",
  "environment": "production",
  "release": "v2.3.1",
  "exception": {
    "type": "PaymentError",
    "value": "upstream timeout after 30s",
    "frames": [
      {
        "function": "processPayment",
        "module": "com.example.payments",
        "file": "PaymentService.java",
        "line": 142,
        "in_app": true
      },
      {
        "function": "handleRequest",
        "module": "com.example.api",
        "file": "ApiHandler.java",
        "line": 89,
        "in_app": true
      }
    ]
  },
  "tags": {
    "user_id": "u_42",
    "region": "eu-west-1"
  }
}
```

| Поле | Обязательное | Описание |
|---|---|---|
| `level` | Нет | `debug` · `info` · `warning` · `error` · `fatal`. По умолчанию `error` |
| `message` | Да | Читаемое описание ошибки |
| `environment` | Нет | Например: `production`, `staging` |
| `release` | Нет | Тег версии отправляющего приложения |
| `exception` | Нет | Структурированный стек; имеет приоритет над `message` при вычислении отпечатка |
| `exception.frames[].in_app` | Нет | Пометить фреймы, принадлежащие коду приложения (остальные считаются vendor) |
| `tags` | Нет | Произвольные метаданные в формате ключ-значение |

Максимальный размер тела запроса: **1 МиБ**.

---

## Алгоритм группировки

Beacon вычисляет **SHA-256** отпечаток для группировки семантически одинаковых ошибок.

**При наличии стека (`exception.frames`):**
1. Берутся фреймы с `in_app: true`; если таких нет — все фреймы
2. Берутся первые 5 фреймов
3. Сигнатура: `ТипИсключения|module:function|module:function|...`

**Fallback по сообщению:**
1. Сообщение нормализуется (замены применяются в указанном порядке):

| Паттерн | Замена |
|---|---|
| `https?://...` | `<url>` |
| `user@domain.tld` | `<email>` |
| UUID (`8-4-4-4-12` hex) | `<uuid>` |
| `0x[0-9a-f]+` | `<addr>` |
| `'...'` / `"..."` | `<str>` |
| `\b\d+\b` | `N` |

2. Сигнатура: `level|нормализованное_сообщение`

Благодаря этому `"user 42 not found"` и `"user 9981 not found"` дают **одинаковый отпечаток** и учитываются как один инцидент.

---

## Конфигурация

Вся конфигурация задаётся через переменные окружения. Скопируйте `.env.example` как отправную точку.

| Переменная | По умолчанию | Описание |
|---|---|---|
| `BEACON_DB_DSN` | *(обязательно)* | Строка подключения к PostgreSQL |
| `BEACON_HTTP_ADDR` | `:8080` | Адрес для прослушивания |
| `BEACON_LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error` |
| `BEACON_PROCESS_INTERVAL` | `2s` | Как часто запускается воркер группировки |
| `BEACON_PROCESS_BATCH` | `100` | Событий за один тик воркера |
| `BEACON_DIGEST_INTERVAL` | `1h` | Интервал дайджеста и проверки всплесков |
| `BEACON_SPIKE_FACTOR` | `5` | Оповещать при текущий_час ≥ factor × предыдущий_час |
| `BEACON_SPIKE_MIN` | `10` | Минимум событий в текущем часу для срабатывания |
| `BEACON_ALERT_COOLDOWN` | `15m` | Минимальный интервал между оповещениями об одном инциденте |
| `BEACON_TELEGRAM_TOKEN` | *(отключено)* | Токен Telegram Bot API; пустое значение отключает уведомления |
| `BEACON_SENTRY_SECRET` | *(отключено)* | Общий секрет для совместимого с Sentry приёма событий |

---

## Структура проекта

```
beacon/
├── cmd/beacon/          # main — точка сборки, ручной DI
├── internal/
│   ├── domain/          # чистые бизнес-типы (без внешних импортов)
│   │   ├── event.go
│   │   ├── issue.go
│   │   ├── project.go
│   │   ├── subscription.go
│   │   └── alert.go
│   ├── usecase/         # бизнес-логика; зависит только от domain и собственных портов
│   │   ├── ingest/      # приём событий, проверка токена
│   │   ├── grouping/    # отпечаток → upsert инцидента → триггер оповещения
│   │   ├── alerting/    # cooldown, обнаружение всплесков, диспетчеризация уведомлений
│   │   ├── digest/      # периодический топ-N отчёт
│   │   ├── issue/       # список + управление статусом
│   │   ├── project/     # создание проекта, генерация токена
│   │   └── subscription/
│   ├── adapter/
│   │   ├── fingerprint/ # SHA-256 финgerprinter + нормализатор сообщений
│   │   ├── ingest/
│   │   │   └── generic/ # JSON → domain.Event парсер
│   │   ├── notify/
│   │   │   └── telegram/# клиент Telegram Bot API
│   │   └── repository/
│   │       └── postgres/ # pgstore — реализации на pgx/v5
│   ├── transport/
│   │   └── http/        # chi роутер, middleware, хендлеры
│   └── infrastructure/
│       ├── config/      # конфигурация из env (caarlos0/env)
│       ├── logger/      # slog JSON handler
│       ├── metrics/     # регистрация Prometheus
│       ├── postgres/    # pgxpool + миграции goose
│       └── scheduler/   # тикер-воркер
├── migrations/          # SQL-файлы goose (встроены через go:embed)
├── deployments/
│   └── docker-compose.yml
├── pkg/retry/           # вспомогательный пакет для повторных попыток
├── Dockerfile           # многоэтапная сборка, финальный образ scratch, непривилегированный пользователь
└── Makefile
```

---

## Разработка

### Требования

- Go 1.25+
- Docker + Docker Compose
- `golangci-lint` (опционально, для линтинга)

### Локальный запуск

```bash
# Запустить только Postgres
docker compose -f deployments/docker-compose.yml up postgres -d

# Скопировать переменные окружения
cp .env.example .env
# При необходимости отредактировать BEACON_DB_DSN

# Запустить (миграции применяются автоматически при старте)
make run
```

### Тесты

```bash
make test                          # все тесты с детектором гонок
go test ./internal/usecase/...     # только юнит-тесты (без БД)
```

Юнит-тесты используют написанные вручную fakes, а не моки. Тесты адаптерного слоя требуют работающего Postgres — предварительно выполните `make docker-up`.

### Линтинг

```bash
make lint
```

### Команды Makefile

| Команда | Описание |
|---|---|
| `make run` | Запустить сервис локально |
| `make build` | Статический бинарь → `./bin/beacon` |
| `make test` | Все тесты с детектором гонок |
| `make lint` | golangci-lint |
| `make docker-up` | Собрать образ и запустить все сервисы |
| `make docker-down` | Остановить сервисы и удалить тома |
| `make migrate-up` | Применить ожидающие миграции |
| `make migrate-down` | Откатить последний батч миграций |

---

## Стек технологий

| Компонент | Выбор | Причина |
|---|---|---|
| Язык | Go 1.25 | |
| HTTP-роутер | [go-chi/chi v5](https://github.com/go-chi/chi) | Composable middleware, идиоматичный stdlib handler |
| База данных | PostgreSQL 16 | JSONB-payload, частичные индексы, `ON CONFLICT DO UPDATE` |
| Драйвер БД | [jackc/pgx v5](https://github.com/jackc/pgx) | Нативный протокол, pgxpool, без рефлексии на горячем пути |
| Миграции | [pressly/goose v3](https://github.com/pressly/goose) | Встроенные SQL-файлы, без глобального состояния (`goose.NewProvider`) |
| Конфигурация | [caarlos0/env v11](https://github.com/caarlos0/env) | Минимальный boilerplate, явная валидация |
| Логирование | stdlib `log/slog` | Структурированные JSON-логи, без внешних зависимостей |
| Метрики | [prometheus/client_golang](https://github.com/prometheus/client_golang) | Стандартный scrape-target |
| Контейнер | scratch + непривилегированный пользователь | Минимальная поверхность атаки; финальный образ ~8 МБ |

---

## Подключение уведомлений Telegram

1. Создать бота через [@BotFather](https://t.me/BotFather) и скопировать токен.
2. Задать `BEACON_TELEGRAM_TOKEN=<токен>` в переменных окружения.
3. Добавить бота в группу или начать диалог напрямую. Получить `chat_id` (например, через `getUpdates`).
4. Подписать проект на этот чат:

```bash
curl -X POST http://localhost:8080/api/v1/projects/{project_id}/subscriptions \
  -H "Content-Type: application/json" \
  -d '{"platform":"telegram","chat_id":"-1001234567890"}'
```

Beacon будет отправлять сообщения в следующих случаях:

| Тип | Когда |
|---|---|
| `[NEW]` | Первое появление отпечатка — новый инцидент |
| `[REGRESSION]` | Закрытый инцидент получил новое событие |
| `[SPIKE]` | Частота событий превысила `SPIKE_FACTOR × SPIKE_MIN` |
| `[DIGEST]` | Периодическая сводка каждые `DIGEST_INTERVAL` |

Оповещения учитывают `ALERT_COOLDOWN` — один и тот же инцидент не сгенерирует более одного оповещения за период cooldown.
