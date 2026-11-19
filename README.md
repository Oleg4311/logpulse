# logpulse

[![CI](https://github.com/Oleg4311/logpulse/actions/workflows/ci.yml/badge.svg)](https://github.com/Oleg4311/logpulse/actions/workflows/ci.yml)

Быстрый анализатор access-логов на Go. Подайте ему лог nginx/Apache или JSON-строки и получите частоту запросов, разбивку по кодам ответа, перцентили задержки, самые частые и самые медленные эндпоинты, а также найденные аномалии (всплески трафика и пачки ошибок 5xx).

**Демо в браузере (Go → WebAssembly):** https://oleg4311.github.io/logpulse/ — анализ идёт локально, логи никуда не отправляются.

```
logpulse report
  requests   1613   (skipped lines: 0)
  rate       avg 5.39 req/s, peak 60.00 req/s (per 1s bucket)
Status codes
  2xx ####################....   85.2%  1374
  5xx ###.....................   11.8%  190
Latency (ms)
  p50 57.0   p90 1520.0   p95 1558.0   p99 1764.0   max 2850.0
Anomalies
  13:03:00  spike  traffic spike: 60 requests (avg 5.4)
  13:03:00  errors 41 of 60 requests are 5xx
```

## Установка

Скачайте бинарник для Windows, Linux или macOS на странице [Releases](https://github.com/Oleg4311/logpulse/releases) или соберите сами:

```
go install github.com/Oleg4311/logpulse/cmd/logpulse@latest
```

## Использование

```
logpulse access.log                    # один или несколько файлов
cat access.log | logpulse              # из stdin
tail -f access.log | logpulse -live    # живая панель, обновляется во время чтения
logpulse -json -top 20 access.log      # отчёт в формате JSON
```

| Флаг | Назначение |
|---|---|
| `-format auto\|nginx\|json` | формат строк, `auto` определяет его для каждой строки |
| `-top N` | сколько эндпоинтов показывать (по умолчанию 10) |
| `-json` | вывести отчёт в JSON |
| `-live` | перерисовывать отчёт дважды в секунду |
| `-no-color` | отключить цвета ANSI |

### Поддерживаемые форматы

- **nginx / Apache combined**, при желании с `request_time` в конце строки (`... "ua" 0.123` или `rt=0.123`): он используется как задержка.
- **JSON-строки** с типовыми именами полей: `time|ts|timestamp|@timestamp`, `method|verb`, `path|uri|url|request_uri`, `status|code`, `bytes|size|body_bytes_sent`; задержка в `duration_ms|latency_ms` (миллисекунды) или `duration|latency|request_time|elapsed` (секунды). Время: RFC 3339 или unix в секундах/миллисекундах.

## Как это работает

- Пути нормализуются (`/users/42?x=1` → `/users/:id`), чтобы эндпоинты группировались.
- Перцентили считаются методом ближайшего ранга.
- Время делится на интервалы (1 с, 1 мин или 1 ч в зависимости от длины лога). Интервал считается **всплеском**, если в нём не менее 5 запросов и их больше среднего плюс 3σ, и **пачкой ошибок**, если не менее половины из ≥ 5 запросов дали 5xx.
- Пакеты анализа (`internal/parse`, `internal/stats`) общие для CLI (`cmd/logpulse`) и сборки WebAssembly (`cmd/wasm`).

## Разработка

```
go test ./...
go run ./cmd/gensample > testdata/sample.log   # пересоздать демо-лог
GOOS=js GOARCH=wasm go build -o logpulse.wasm ./cmd/wasm
```

GitHub Actions: `ci.yml` (vet и тесты на Linux/Windows/macOS), `pages.yml` (собирает WASM-демо и публикует на Pages), `release.yml` (тег `v*` → бинарники под все платформы в Releases).

## Лицензия

MIT
