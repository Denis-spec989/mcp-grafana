# 🤖 Grafana MCP

MCP-сервер для **Grafana**, который подключает AI-инструменты к Grafana через [Model Context Protocol](https://modelcontextprotocol.io/). Позволяет AI-агентам находить дашборды, запрашивать метрики и логи, проверять алерты, исследовать инциденты и управлять ресурсами через взаимодействие на естественном языке.

Примеры использования:

> «Найди дашборд сервиса payments»\
> «Покажи p95 времени ответа за последний час»\
> «Найди ошибки в логах backend за последние 15 минут»\
> «Какие алерты сейчас срабатывают и кто дежурит?»\
> «Добавь панель с RPS на дашборд сервиса»

AI-агент обращается к Grafana и её источникам данных, получает нужный контекст и возвращает ответ. Доступные действия зависят от включённых инструментов и прав аккаунта.

---

## 📋 Оглавление

- [Возможности](#-возможности)
- [Быстрый старт](#-быстрый-старт)
- [Конфигурация](#-конфигурация)
- [Аутентификация](#-аутентификация)
- [Инструменты](#-инструменты)
- [Фильтрация инструментов](#-конфигурация-инструментов)
- [Безопасность](#-безопасность)
- [Мониторинг сервера](#-мониторинг-сервера)
- [Troubleshooting](#-troubleshooting)
- [Разработка](#-разработка)
- [Лицензия](#-лицензия)

---

## ✨ Возможности

- 🛠️ **Инструменты Grafana** — дашборды, папки, источники данных, алерты, аннотации, snapshots и плагины.
- 📊 **Метрики, логи и профили** — Prometheus, Loki и Pyroscope; дополнительные категории для InfluxDB, ClickHouse, CloudWatch, Graphite, Athena, Snowflake, Elasticsearch/OpenSearch и Quickwit.
- 🔌 **Подключение через stdio** — MCP-клиент запускает локально собранный бинарный файл.
- 🧠 **Dynamic Instructions** — инструкции для AI-агента формируются с учётом включённых категорий инструментов.
- 🎛️ **Фильтрация инструментов** — выбор категорий через `--enabled-tools`, отключение через `--disable-<category>` и режим `--disable-write`.
- 📦 **Экономия контекста** — краткая сводка дашборда, выборка свойств через JSONPath и точечные изменения вместо передачи полного JSON.
- 🚨 **Расследование инцидентов** — Grafana Incident, Sift, OnCall и Asserts при наличии соответствующих сервисов и прав.
- 🔐 **Аутентификация** — service account token, чтение токена из файла и Basic Auth.
- 🖼️ **PNG-изображения** — рендеринг дашбордов и панелей через Grafana Image Renderer, включая preview из provisioning-репозитория.
- 📈 **Локальная диагностика** — stderr-логи и журнал медленных запросов.

---

## 🚀 Быстрый старт

### 1. Предварительные требования

- **Grafana 9.0+**. Отдельные инструменты требуют дополнительных API, плагинов или сервисов.
- **Service account token** с правами на нужные ресурсы (см. [Аутентификация](#-аутентификация)).
- Предустановленный **GigaCode CLI** с поддержкой MCP и настроенным секретом `grafana/GRAFANA_SERVICE_ACCOUNT_TOKEN`.
- **Go 1.26.3+** и **Git** для сборки бинарного файла.

### 2. Соберите бинарный файл

> **Важно:** Установка должна выполняться в директории **`work`**, где разрешён запуск скриптов и исполняемых файлов. Размещайте репозиторий и собранный бинарный файл внутри неё, например: `/home/work/22808950@sigma.sbrf.ru/GigaIdeProject/mcp-grafana`.

Скачайте репозиторий и выполните сборку:

```bash
git clone https://github.com/grafana/mcp-grafana.git
cd mcp-grafana
go build -o dist/mcp-grafana ./cmd/mcp-grafana
```

Бинарный файл появится в `dist/mcp-grafana`. Если репозиторий уже скачан, выполните команду сборки из его корня. При установленном Make можно использовать `make build`.

### 3. Подключение в stdio-режиме

Добавьте сервер `grafana` в раздел `mcpServers` конфигурации GigaCode CLI:

```json
{
  "mcpServers": {
    "grafana": {
      "command": "/home/work/22808950@sigma.sbrf.ru/GigaIdeProject/mcp-grafana/dist/mcp-grafana",
      "args": [
        "--transport",
        "stdio"
      ],
      "env": {
        "GRAFANA_URL": "http://host:3000",
        "GRAFANA_SERVICE_ACCOUNT_TOKEN": "$secret:grafana/GRAFANA_SERVICE_ACCOUNT_TOKEN"
      },
      "timeout": 600000
    }
  }
}
```

- `command` — абсолютный путь к собранному бинарному файлу. Замените его, если репозиторий находится в другом каталоге.
- `args` — аргументы запуска; `--transport stdio` задаёт подключение через стандартные потоки процесса.
- `GRAFANA_URL` — адрес вашей Grafana. Замените `host` на имя сервера.
- `$secret:grafana/GRAFANA_SERVICE_ACCOUNT_TOKEN` — ссылка на секрет с токеном, настроенный в GigaCode CLI.
- `timeout: 600000` — таймаут MCP-клиента, равный 10 минутам.

GigaCode CLI выступает MCP-клиентом: самостоятельно запускает бинарный файл и обменивается с ним MCP-сообщениями через stdio.

---

## ⚙️ Конфигурация

### Переменные окружения

Передаются бинарному файлу через блок `env` в конфигурации GigaCode CLI.

| Переменная | Обязательная | По умолчанию | Описание |
|---|---|---|---|
| `GRAFANA_URL` | для удалённой Grafana | `http://localhost:3000` | Базовый URL Grafana |
| `GRAFANA_SERVICE_ACCOUNT_TOKEN` | для token auth | — | Токен service account |
| `GRAFANA_SERVICE_ACCOUNT_TOKEN_FILE` | нет | — | Путь к файлу с токеном; используется, если inline-токен не задан |
| `GRAFANA_USERNAME` | для Basic Auth | — | Имя пользователя Grafana |
| `GRAFANA_PASSWORD` | для Basic Auth | — | Пароль пользователя Grafana |
| `GRAFANA_ORG_ID` | нет | организация по умолчанию | Числовой ID организации Grafana |
| `GRAFANA_EXTRA_HEADERS` | нет | — | Дополнительные заголовки запросов к Grafana в виде JSON-объекта |

> `GRAFANA_API_KEY` поддерживается для обратной совместимости, но устарел. Используйте `GRAFANA_SERVICE_ACCOUNT_TOKEN`.

Пример `GRAFANA_EXTRA_HEADERS`: `{"X-Tenant-ID":"tenant-123","X-Custom-Header":"value"}`.

### Основные CLI-флаги

| Флаг | По умолчанию | Описание |
|---|---|---|
| `--transport` | `stdio` | В конфигурации GigaCode CLI явно укажите `--transport stdio` |
| `--enabled-tools` | см. [Фильтрация инструментов](#-конфигурация-инструментов) | Список категорий через запятую, заменяющий стандартный набор |
| `--disable-<category>` | `false` | Отключить категорию, например `--disable-oncall` |
| `--disable-write` | `false` | Ограничить встроенные операции записи |
| `--max-loki-log-limit` | `100` | Максимум строк в одном ответе `query_loki_logs` |
| `--log-level` | `info` | Уровень логирования: `debug`, `info`, `warn`, `error` |
| `--debug` | `false` | Подробные HTTP-логи запросов к Grafana; включает уровень `debug` |
| `--slow-request-threshold` | `0` | Порог журнала медленных MCP-запросов, например `500ms` или `5s`; `0` отключает |
| `--slow-request-log-level` | `warn` | Уровень медленных запросов: `info` или `warn` |
| `--version` | — | Вывести версию и завершить работу |

> Для определения усечения ответа Loki инструмент запрашивает `limit + 1` строк. Задавайте `--max-loki-log-limit` минимум на 1 меньше серверного `max_entries_limit_per_query` Loki.

TLS-флаги приведены в разделе [Безопасность](#-безопасность). Все параметры собранного бинарного файла доступны через `./dist/mcp-grafana --help`.

---

## 🔑 Аутентификация

### Service account token (рекомендуется)

1. Откройте Grafana и перейдите в **Administration → Users and access → Service accounts**.
2. Создайте service account и назначьте права на нужные ресурсы.
3. Создайте токен и сохраните его.
4. Сохраните токен в секрете GigaCode CLI `grafana/GRAFANA_SERVICE_ACCOUNT_TOKEN`. В блоке `env` используйте ссылку `$secret:grafana/GRAFANA_SERVICE_ACCOUNT_TOKEN`.

Подробная инструкция: [Grafana service accounts](https://grafana.com/docs/grafana/latest/administration/service-accounts/#add-a-token-to-a-service-account-in-grafana).

Для чтения нужны соответствующие read/query-права; для изменения ресурсов — write-права. RBAC проверяет **действие и scope**: например, `datasources:query` со scope `datasources:uid:prometheus-prod` или `dashboards:read` со scope `dashboards:uid:monitoring-dashboard`.

Встроенная роль `Editor` даёт широкий доступ на чтение и запись. Для ограниченного доступа назначайте только необходимые права. Grafana Incident и Sift используют базовые роли: `Viewer` для чтения и `Editor` для операций записи.

### Токен из файла

Задайте `GRAFANA_SERVICE_ACCOUNT_TOKEN_FILE`, например `/var/run/secrets/grafana/token`. Файл перечитывается при запросах; ротация токена не требует перезапуска сервера. Пробелы и перевод строки по краям удаляются.

Если одновременно задан `GRAFANA_SERVICE_ACCOUNT_TOKEN`, он имеет приоритет.

### Basic Auth

Вместо токена задайте `GRAFANA_USERNAME` и `GRAFANA_PASSWORD`. Для выбора организации используйте `GRAFANA_ORG_ID`; это не расширяет права аккаунта.

---

## 🛠️ Инструменты

Ниже перечислены встроенные инструменты. Набор, доступный клиенту, зависит от CLI-флагов, прав Grafana и установленных интеграций.

### 📖 Чтение и диагностика

| Инструмент | Описание | Пример запроса |
|---|---|---|
| `search_dashboards` | Поиск дашбордов | «Найди дашборд payments» |
| `search_folders` | Поиск папок | «Покажи папки команды backend» |
| `get_dashboard_summary` | Краткая сводка без полного JSON | «Какие панели есть на дашборде?» |
| `get_dashboard_property` | Выборка свойств через JSONPath | «Покажи названия панелей» |
| `get_dashboard_by_uid` | Полный JSON дашборда | «Покажи конфигурацию дашборда по UID» |
| `get_dashboard_panel_queries` | Запросы панелей и их datasources | «Какие запросы использует дашборд?» |
| `list_datasources`, `get_datasource` | Список и параметры источников данных | «Какие Prometheus подключены?» |
| `query_prometheus` | Instant/range-запросы PromQL | «Покажи RPS за последний час» |
| `query_prometheus_histogram` | Процентили гистограмм | «Рассчитай p95 и p99 latency» |
| `list_prometheus_metric_names` | Имена метрик | «Найди метрики HTTP-запросов» |
| `list_prometheus_metric_metadata` | Типы и описания метрик | «Что означает эта метрика?» |
| `list_prometheus_label_names`, `list_prometheus_label_values` | Метки и их значения | «Какие значения есть у label service?» |
| `query_loki_logs` | Логи и метрические запросы LogQL | «Найди ошибки backend за 15 минут» |
| `list_loki_label_names`, `list_loki_label_values` | Метки логов | «Какие приложения отправляют логи?» |
| `query_loki_stats` | Статистика потоков логов | «Сколько данных попадает под этот selector?» |
| `query_loki_patterns` | Повторяющиеся шаблоны логов | «Покажи типичные сообщения сервиса» |
| `analyze_loki_labels` | Анализ качества и кардинальности меток | «Проверь метки Loki» |
| `list_pyroscope_profile_types` | Доступные типы профилей | «Какие профили доступны?» |
| `list_pyroscope_label_names`, `list_pyroscope_label_values` | Метки профилей | «Какие сервисы профилируются?» |
| `query_pyroscope` | Профили и метрики Pyroscope | «Покажи профиль CPU сервиса» |
| `alerting_manage_routing` | Notification policies, contact points и time intervals | «Куда отправляются уведомления об алертах?» |
| `list_incidents`, `get_incident` | Поиск и детали инцидентов | «Покажи активные инциденты» |
| `list_sift_investigations`, `get_sift_investigation`, `get_sift_analysis` | Расследования и результаты Sift | «Что обнаружило расследование?» |
| `list_oncall_schedules`, `get_oncall_shift`, `get_current_oncall_users` | Расписания, смены и дежурные | «Кто сейчас дежурит?» |
| `list_oncall_teams`, `list_oncall_users` | Команды и пользователи OnCall | «Покажи команды OnCall» |
| `list_alert_groups`, `get_alert_group` | Группы оповещений OnCall | «Покажи открытые alert groups» |
| `get_assertions` | Сводка Asserts для сущности | «Покажи assertions для сервиса» |
| `get_annotations`, `get_annotation_tags` | Аннотации и теги | «Какие деплои отмечены на дашборде?» |
| `list_snapshots`, `get_snapshot` | Снимки дашбордов | «Покажи сохранённые snapshots» |
| `get_panel_image` | PNG панели или дашборда | «Сделай изображение панели latency» |
| `list_provisioning_repositories` | Репозитории provisioning и состояние синхронизации | «Какие git-sync репозитории подключены?» |
| `validate_provisioning_file` | Проверка файла provisioning без применения | «Проверь дашборд из ветки feature» |
| `get_plugin`, `search_plugin_information` | Проверка плагина и поиск в каталоге | «Найди плагин для ClickHouse» |
| `suggest_loki_alloy_label_config` | Генерация фрагмента Alloy для правил меток | «Подготовь конфигурацию допустимых labels» |

### ✏️ Запись

Эти инструменты отключаются флагом `--disable-write`.

| Инструмент | Описание | Пример запроса |
|---|---|---|
| `update_dashboard` | Создание, полная замена или точечное изменение дашборда | «Добавь панель RPS на дашборд» |
| `create_folder` | Создание папки | «Создай папку команды backend» |
| `create_incident` | Создание инцидента | «Создай инцидент о недоступности API» |
| `add_activity_to_incident` | Запись в журнал инцидента | «Добавь результат проверки в инцидент» |
| `create_annotation` | Создание аннотации, включая Graphite-формат | «Отметь время деплоя на дашборде» |
| `update_annotation` | Частичное обновление аннотации | «Исправь текст аннотации» |
| `find_error_pattern_logs` | Запуск Sift-анализа ошибок с созданием расследования | «Исследуй рост ошибок в логах» |
| `find_slow_requests` | Запуск Sift-анализа медленных запросов | «Найди медленные запросы в Tempo» |
| `create_snapshot`, `delete_snapshot` | Создание и удаление снимков | «Создай snapshot дашборда» |
| `get_panel_image_and_save` | Сохранение PNG на машине MCP-сервера | «Сохрани панель в файл» |
| `install_plugin` | Установка плагина в Grafana | «Установи плагин по его ID» |

### 🔀 Инструменты с режимами чтения и записи

| Инструмент | Обычный режим | С `--disable-write` |
|---|---|---|
| `alerting_manage_rules` | Просмотр, версии, создание, изменение и удаление alert rules | Только операции чтения |
| `grafana_api_request` | Запросы к Grafana API с опциональным jq-фильтром ответа | Только GET |
| `generate_deeplink` | Ссылки на дашборды, панели и Explore; опциональное сокращение URL | Полные ссылки; `shorten=true` игнорируется |

### 🧩 Дополнительные категории

По умолчанию отключены. Для подключения укажите нужную категорию в `--enabled-tools`.

| Категория | Инструменты | Назначение |
|---|---|---|
| `runpanelquery` | `run_panel_query` | Выполнение запросов панелей с диапазоном времени и переменными |
| `examples` | `get_query_examples` | Примеры запросов для разных datasources |
| `influxdb` | `query_influxdb` | InfluxQL (v1) и Flux (v2) |
| `clickhouse` | `list_clickhouse_tables`, `describe_clickhouse_table`, `query_clickhouse` | Таблицы, схемы и SQL с макросами Grafana |
| `cloudwatch` | `list_cloudwatch_namespaces`, `list_cloudwatch_metrics`, `list_cloudwatch_dimensions`, `query_cloudwatch` | Метрики AWS CloudWatch |
| `graphite` | `query_graphite`, `list_graphite_metrics`, `list_graphite_tags`, `query_graphite_density` | Метрики и теги Graphite |
| `athena` | `list_athena_catalogs`, `list_athena_databases`, `list_athena_tables`, `describe_athena_table`, `query_athena` | Каталоги, схемы и SQL Amazon Athena |
| `snowflake` | `list_snowflake_tables`, `describe_snowflake_table`, `query_snowflake` | Таблицы и SQL Snowflake, включая `SNOWFLAKE.TELEMETRY.EVENTS` |
| `elasticsearch` | `query_elasticsearch` | Elasticsearch/OpenSearch: Lucene и Query DSL |
| `quickwit` | `query_quickwit` | Поиск логов и документов в Quickwit |
| `admin` | `list_teams`, `list_users_by_org`, `list_all_roles`, `get_role_details`, `get_role_assignments`, `list_user_roles`, `list_team_roles`, `get_resource_permissions`, `get_resource_description` | Пользователи, команды, роли и права |

Для SQL-источников запросы идут через datasource Grafana: доступ к базе настраивается в самой Grafana. Snowflake требует соответствующего Enterprise datasource plugin.

Справочная таблица инструментов и RBAC: [MCP tools reference](docs/sources/reference/mcp-tools-table.md).

> **Экономия контекста:** Начинайте с `get_dashboard_summary`. Для отдельных свойств используйте `get_dashboard_property`, для небольших правок — `update_dashboard` с `uid` и `operations`. Полный JSON через `get_dashboard_by_uid` нужен только при работе со всей конфигурацией.

---

## 🎛️ Конфигурация инструментов

Инструменты выбираются **по категориям через CLI-флаги**. Значение `--enabled-tools` полностью заменяет стандартный список; `--disable-<category>` имеет приоритет и отключает категорию даже при её наличии в списке.

Стандартные категории встроенных инструментов:

```text
search,datasource,incident,prometheus,loki,alerting,dashboard,folder,oncall,asserts,sift,pyroscope,navigation,annotations,rendering,snapshot,plugin,api,config,provisioning
```

Оставить основные встроенные инструменты для диагностики:

```bash
./dist/mcp-grafana --transport stdio --enabled-tools search,datasource,dashboard,prometheus,loki,alerting --disable-write
```

Отключить отдельные категории, сохранив остальные стандартные:

```bash
./dist/mcp-grafana --transport stdio --disable-oncall --disable-sift --disable-asserts
```

Сохранить стандартные категории и добавить выполнение запросов панелей:

```bash
./dist/mcp-grafana --transport stdio --enabled-tools search,datasource,incident,prometheus,loki,alerting,dashboard,folder,oncall,asserts,sift,pyroscope,navigation,annotations,rendering,snapshot,plugin,api,config,provisioning,runpanelquery
```

В конфигурации GigaCode CLI добавляйте флаги в массив `args`, сохраняя `"--transport", "stdio"`. Например, для ограничения записи: `"args": ["--transport", "stdio", "--disable-write"]`.

> **Режим чтения:** `--disable-write` ограничивает встроенные операции записи. Права в Grafana и источниках данных настраиваются отдельно.

---

## 🔒 Безопасность

- 🔑 **Права доступа** — выдавайте service account только необходимые действия и scopes; ограничивайте доступ к конкретным dashboards, folders и datasources.
- 🎛️ **Ограничение действий** — используйте `--disable-write` и включайте только нужные категории инструментов.
- 🔐 **TLS к Grafana** — проверка сертификата включена по умолчанию. Для корпоративного CA используйте `--tls-ca-file`; для mTLS добавьте клиентский сертификат и ключ.
- 🙈 **Debug-логи** — значения заголовков `Authorization`, `X-Access-Token`, `X-Grafana-Id` и `Cookie` маскируются. Это не гарантирует скрытие всех чувствительных данных в ответах Grafana или произвольных дополнительных заголовках.
- 📁 **Локальные файлы** — `get_panel_image_and_save` сохраняет PNG на машине, где GigaCode CLI запускает бинарный файл.

### TLS-конфигурация

| Флаг | По умолчанию | Назначение |
|---|---|---|
| `--tls-ca-file` | системные CA | CA для проверки сертификата Grafana |
| `--tls-cert-file` | — | Клиентский сертификат для mTLS к Grafana |
| `--tls-key-file` | — | Клиентский приватный ключ для mTLS к Grafana |
| `--tls-skip-verify` | `false` | Отключить проверку сертификата Grafana; только для тестирования |

Пример запуска бинарного файла с корпоративным CA для соединения с Grafana:

```bash
./dist/mcp-grafana --transport stdio --tls-ca-file /path/to/grafana-ca.crt
```

Для запуска из GigaCode CLI добавьте `"--tls-ca-file", "/path/to/grafana-ca.crt"` в массив `args`. URL Grafana и credentials передаются через `env`.

### Логи и телеметрия

Сервер пишет логи в **stderr** и не отправляет собственные логи, traces, события использования или метрики в удалённый telemetry collector, включая запуск и завершение работы. Переменные `OTEL_EXPORTER_OTLP_*` не включают удалённый экспорт; CLI-флага для него нет.

Эта политика относится к телеметрии MCP-процесса. Рабочие запросы к Grafana и публичному каталогу плагинов, а также ответы MCP-клиенту выполняются по назначению. Передача данных самим AI-клиентом, Grafana, системными агентами и внешним сборщиком логов настраивается отдельно. При использовании Go SDK ответственность за подключённые logger, transport и OpenTelemetry providers лежит на вызывающем приложении.

---

## 📈 Мониторинг сервера

### Логи

Диагностика процесса доступна через stderr-логи в MCP-клиенте. Уровень задаётся флагом `--log-level`; для подробных запросов к Grafana используйте `--debug`.

### Медленные запросы

```bash
./dist/mcp-grafana --transport stdio --slow-request-threshold 500ms --slow-request-log-level warn
```

Для использования в GigaCode CLI добавьте эти флаги в массив `args`. Запись содержит метод MCP, длительность, порог, имя инструмента для `tools/call` и сведения об ошибке при наличии.

---

## ❓ Troubleshooting

| Симптом | Возможная причина | Решение |
|---|---|---|
| Бинарный файл не найден | Файл не собран или указан неверный путь в `command` | Выполните сборку и укажите абсолютный путь к `dist/mcp-grafana` |
| `Permission denied` при запуске | Нет права на выполнение файла или доступа к каталогу | Проверьте права пользователя, от которого запущен GigaCode CLI, на бинарный файл и каталог |
| MCP-сервер недоступен в GigaCode CLI | Ошибка JSON или неверный формат конфигурации клиента | Проверьте синтаксис, путь к программе и логи GigaCode CLI |
| `401 Unauthorized` | Неверный, истёкший или отозванный токен | Проверьте токен и URL Grafana; создайте новый токен при необходимости |
| `403 Forbidden` | Не хватает прав или выбран неверный scope/organization | Проверьте роль, RBAC-действия, scopes и `GRAFANA_ORG_ID` |
| `id is invalid` при получении datasource по UID | Несовместимая версия Grafana | Используйте Grafana 9.0+; проверьте версию и endpoint |
| Ошибка проверки TLS-сертификата | Недоверенный CA или неверное имя сервера | Передайте CA через `--tls-ca-file` и проверьте hostname сертификата |
| Grafana недоступна | Неверный адрес или нет сетевого доступа с машины, где запущен GigaCode CLI | Проверьте `GRAFANA_URL` и доступность Grafana с машины, где запускается бинарный файл |
| Нет ClickHouse, Snowflake или других дополнительных инструментов | Категория выключена по умолчанию | Добавьте категорию в полный список `--enabled-tools` |
| Нет инструментов записи | Передан `--disable-write` или отключена категория | Проверьте аргументы запуска и права Grafana |
| Не получается получить PNG | Grafana Image Renderer не настроен | Установите и настройте renderer; проверьте доступ к дашборду |
| Пустой результат запроса | Неверные datasource, labels, переменные или время | Проверьте запрос в Explore и укажите часовой пояс; время без offset трактуется как UTC |
| Логи Loki обрезаны | Достигнут лимит строк | Уточните selector/диапазон или настройте `--max-loki-log-limit` с учётом серверного лимита |

Для диагностики добавьте `--debug` или `--slow-request-threshold 500ms`. Учитывайте, что логи могут содержать данные ответов Grafana.

---

## 🏗️ Разработка

Требуются **Go 1.26.3+** и Make.

| Команда | Описание |
|---|---|
| `make build` | Сборка бинарного файла в `dist/mcp-grafana` |
| `make test`, `make test-unit` | Unit-тесты без внешних сервисов |
| `make lint` | Проверки Go, JSONSchema-тегов и OpenAPI context propagation |
| `make lint-jsonschema`, `make lint-openapi` | Отдельный запуск внутренних линтеров |

Запуск unit-тестов без Make:

```bash
go test -v -tags unit ./...
```

Для подключения к Grafana соберите бинарный файл и настройте его запуск из GigaCode CLI, как описано в [Быстром старте](#-быстрый-старт).

### Стек

- **Go** — сервер и инструменты.
- **mcp-go** — MCP-протокол и транспорты.
- **Grafana OpenAPI client** — доступ к Grafana API.
- **JSONSchema** — схемы параметров инструментов.
- **Go testing** — тестирование.
- **golangci-lint и внутренние линтеры** — проверки кода.

Подробнее: [Build and test](docs/sources/developer/build-and-test.md), [Go SDK](docs/sources/developer/go-sdk.md), [JSONSchema linter](internal/linter/jsonschema/README.md).
