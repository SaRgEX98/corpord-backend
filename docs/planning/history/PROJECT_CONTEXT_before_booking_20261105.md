# Corpord: контекст для продолжения работы

Обновлено: 2026-11-05, Asia/Yekaterinburg. Это сохранённый снимок состояния,
а не подтверждение будущего состояния GitHub или запущенных сервисов.

## Обновление локального dev: 2026-11-05, завершено

Пользователь разрешил pull и проверку после merge, затем продолжение после
прерывания. PR backend15/frontend7 MERGED 2026-11-03. Актуальные dev:
backend `38f5f71`, frontend `df949be`; fresh pull 2026-11-05 — already up to date.
Основной frontend обновлён. Для backend создан отдельный `corpord-backend-dev`
на ветке dev; пользовательский dirty `corpord-backend`/local/seed-work-preserved
остался нетронутым. Содержимое squash commits совпадает с проверенными PR HEAD.

GHCR dev образы скачаны, OCI revisions совпадают с этими SHA. До миграции
сохранена/проверена новая custom-format копия
`corpord-infra/backups/before-catalog-dev-20261105.dump` (76602 B, mode0600).
Прежняя копия 20261103 также сохранена; содержимое/секреты не выводились.
До обновления:4 users,6 buses,10 drivers,14 stops,56 trips,224 trip_stops.
После наполнения сохранена `corpord-infra/backups/after-catalog-dev-20261105.dump`
(38304273 B,mode0600), проверены list и полное чтение `pg_restore --file=/dev/null`;
данные/SQL архива в вывод не попадали, восстановления в действующую БД не было.

Запущен основной Compose **с** `docker-compose.geo-dev.yaml`, как в прежнем
окружении (Photon Екатеринбурга и собственная карта сохранены). Все6 постоянных
сервисов healthy, migrate exit0, Goose20261103000100. Backend38f5f71/frontenddf949be
подтверждены в запущенных контейнерах, localhost3000 и catalog API8080 отвечают200.
Настоящие env не менялись. Другие проекты Docker не менялись.

Dry-run национального импорта в dev с ROLLBACK прошёл без конфликтов, после него
импорт применён:2680 новых city/town,162512 остановок,14 существующих городов
связаны без присвоения владения, только отсутствующие координаты дополнены.
Снимок сохранности существующих записей — только counts/digests вне Git,
`corpord-catalog-data/dev-baseline-20261105.json`, без записей аккаунтов.
Runner вне Git `dev_catalog.py` получает подключение из Docker только в памяти,
не печатает секреты. Данные russia-v2 неизменны. Сгенерирован dataset russia-demo-v1:
10000 направленных связей,2 отправления/сутки,30 дней2026-11-06—2026-12-05,
600000 новых demo trips /1336680 trip_stops. Повторы импорта/диапазона:
0 созданных/изменённых строк,0 новых дней. Импорт при старте API не запускается.

Итог в основной dev-БД:4 users,10006 buses,20010 drivers,162526 stops,
600056 trips,1336904 trip_stops,2694 city/town,85 регионов. Прежние4 users,
6 buses,10 drivers,14 stops,56 trips,224 trip_stops не изменились — digests
всех исходных групп совпали. Тестовые аккаунты в этой БД не создавались.
Пересечений автобусов/водителей0,несовпадений demo fare0,демоостановок без
города/пояса0. Размер с индексами1007785651B≈961MiB/1.01GB, отдельно от
WAL/backups/Photon/карт.58316 остановок остаются с unknown городом.

Проверки этого развёртывания:
- Изолированный E2E сначала остановился на локальной сборке из-за старого
  node_modules без maplibre-gl. npm ci из закреплённого lock исправил окружение;
  исходники/lock не менялись. Typecheck,48 unit/contract и production build PASS.
- Изолированные PG/Redis/аккаунты + скачанный GHCR backend:6 PASS/4 intentional
  skip (геостек/национальный отдельный тест). Девять CRUD,ACL,цены,calendar/quote,
  страницы/устаревшие ответы/регион. Временные контейнеры/сети удалены.
- Основные GHCR frontend/backend:9 HTTP checks и read-only Chromium PASS,
  без API mocks/создания аккаунтов. Реальные страницы30→60 без per-row fetch,
  регион→город,demo search/calendar/quote,обе темы/mobile390 без overflow/pageerror.
  Настоящие Photon/PMTiles работают, карта загружается по кнопке и меняет тему;
  публичных геозапросов0. Скриншоты осмотрены, лежат вне Git.
- Основной dev, HTTP warm cache,concurrency4:312/312 без ошибок,warm-up также.
  p95 city name77.67ms,prefix14.96ms,одна буква34.77ms,опечатка76.60ms,
  регион4.40ms,stop→stop8.99ms,city→city68.39ms,цена65.34ms,calendar70.36ms.
  Это не SLA; основной PG не имеет ограничений изолированного пилота3Nov.
- GitHub dev test/build/publish PASS, SHA совпадают с запущенными:
  backend https://github.com/SaRgEX98/corpord-backend/actions/runs/41646461038 ;
  frontend https://github.com/SaRgEX98/corpord-frontend/actions/runs/41646298553 .

Приложение **http://localhost:3000**, API **http://localhost:8080/api/v1**.
Для последующих операций сохранять оба Compose-файла, включая geo-dev override.
Geo по-прежнему Екатеринбург, не национальные Photon/карты. Все6 сервисов healthy;
migrate exit0. Изолированные catalog-test PG/Redis сейчас остановлены, данные
сохранены. Основной backend dirty оставлен нетронутым, backend-dev и frontend
dev чистые. Актуальное развёртывание описано в `corpord-infra/DEPLOYMENT.md`.

Следующие задачи: сверка официальной полноты/unknown mapping и группировки
station-platform, реальное расписание/тарифы, retention, бюджет WAL/backups/нагрузки.
Ежедневное автопродление, бронирование/платежи и национальный геостек не включены.

## Общероссийский справочник и lazy loading: 2026-11-03

Этот раздел актуальнее исторических разделов ниже. PR backend14/frontend6
проверены через GitHub: MERGED 2026-11-01, соответственно 09:26:52Z/09:27:04Z.
Свежий fetch dev: backend `4858292`, frontend `bfbbe55`; новые изменения выполнены
в отдельных worktree, без изменения пользовательского dirty основного backend.

| Репозиторий / worktree | Ветка / опубликованный HEAD | PR в dev |
| --- | --- | --- |
| `corpord-backend-catalog` | `feat/russia-catalog`, `5a80b8c` | https://github.com/SaRgEX98/corpord-backend/pull/15 |
| `corpord-frontend-catalog` | `feat/russia-catalog`, `809c2aa` | https://github.com/SaRgEX98/corpord-frontend/pull/7 |

Оба PR draft для ревью; merge/deploy не выполнялись. Worktree после публикации
чистые. Fetch/push через HTTPS + существующий gh credential helper, сохранённые
remote не менялись. Основной backend по-прежнему содержит пользовательские
Dockerfile/Makefile/user.go и seed/roles-изменения, в новые коммиты они не попали.

Данные и инструменты вне Git: `corpord-catalog-data`. Закреплённый Geofabrik
`russia-261102.osm.pbf`, 4 194 615 984 B, опубликованный MD5
`725964e01f0e1a65acd92ef4e893c48d`; SHA-256
`cb3713f08c0359c283537a7e6d7e50f4fee4c29b848b474f836f0a45c9dc5105`.
Актуальный подготовленный набор `russia-v2`, SHA-256 JSONL
`b7e4ea2964b84da4743566ed827b5eaaf8a8c066914500ca2df5788aa185ef0f`.
`russia-v1` и Ural наборы — промежуточные, не итоговый национальный каталог.
Профиль city/town + именованные bus stations/stops/явные bus platforms;
85 регионов охвата Geofabrik (исходные OSM-коды RU-* и UA-43/UA-40 сохраняются).
Это географический профиль источника, не смена ISO-классификации и не
подтверждённая полнота официального реестра. Сёла/деревни/все адреса не входят.

В отдельной БД `corpord_catalog_russia`: 2 694 city/town, 162 512 остановок,
104 196 с подтверждённым городом / 58 316 unknown mapping. 81 city/town без
подтверждённого региона оставлены в review; 46 060 безымянных остановочных
объектов и 937 stop ways без area пропущены с причинами. Сопоставление только
по точной геометрии/явному однозначному имени, не ближайшему городу.
TimezoneFinder8.3 + полный timezonefinder-data1.2026.3: географические IANA ID;
прежний сокращённый набор мог отдавать Asia/Karachi для Урала и запрещён.
Снимки/provenance, holes/границы, NULL unknown, source-key идентичность,
защита ручных правок и dry-run описаны в backend `docs/CATALOG_IMPORT.md`.

Dataset `russia-demo-v1`: 10 000 направленных связей, 2 отправления/сутки,
30 дней с 2026-11-04, 600 000 рейсов / 1 336 680 trip_stops,
10 000 тестовых автобусов / 20 000 водителей. Расписание/цены синтетические,
`is_demo=true` во всех ответах/календаре/интерфейсе; остатки мест unknown.
Проверка всего набора: 0 пересечений автобусов, 0 пересечений водителей,
0 несовпадений суммы участков и цены, 0 аккаунтов. Повторные импорт/генерация
того же диапазона создают/изменяют 0 записей. Полный граф пар не материализуется.

Подготовка107.87s/maxRSS2 528 408KiB; импорт112.16s/79 400KiB;
генерация195.3s/138 016KiB. Размер БД с индексами 939 103 923B ≈896MiB/0.94GB;
PBF4.19GB отдельно. Не включает Photon/карты, WAL, backups/реплики/VACUUM запас.
При той же плотности +сутки ориентировочно20MB, год порядка7.5GB — линейная
оценка, год не импортировался. Автоматического retention/TTL нет.

Backend миграция `20261103000100`: pg_trgm в public, синхронный catalog_search,
prefix/region/city B-tree, GiST KNN/GIN, индекс отправления. Каталог1SQL,
поиск3SQL независимо от размера страницы; bounded shortlist, keyset cursor,
без COUNT(*) на страницу. Национальный прогон выявил неверные планы PostgreSQL:
принудительные selective LATERAL/OFFSET0 перед расширением рейсов, индекс
`stops(city_id,id)` до EXISTS и `jit=off` для интерактивных API-соединений.
Read-only `cmd/catalog-plan` фиксирует фактический EXPLAIN. Таймауты/отмена.
**Совместимость:** старые array endpoints теперь максимум100, stops города50;
клиенты переходят на `/catalog/:kind`. Пользователи только admin ACL. Backend
выкатывать до frontend. Миграции не запускают импорт автоматически. Rollback
откажется при NULL timezone/дублях имени-региона, несовместимых со старой схемой.

Frontend: страницы30 и окно200 в DOM, связанные варианты только при фокусе,
окно100, выбранные значения отдельно; debounce250ms, abort/generation guard,
ограниченный публичный LRU32/TTL30s. Серверные фильтры, регион уточняет город,
полные остановки только в деталях, карта/календарь lazy chunks, overview одна
приблизительная сводка. Attribution OpenStreetMap/ODbL в общем footer.

Актуальные проверки новых изменений:
- Полный Go race с отдельными PG17/Redis7 и go vet PASS; Python3.14 5/5 PASS
  (интеграции с `TEST_DATABASE_URL`, без пропусков); миграции/rollback,
  постоянное число SQL, cursor scope, ACL, literal wildcard, триггеры, UTC JSON.
- Node26.1 typecheck,48/48 unit/contract,production build PASS.
- Изолированный Chromium из production build:6 PASS/4 intentional skip
  (3 гео/map без геостека + отдельный национальный тест).
- Национальный read-only Chromium:1 PASS без API mocks/аккаунтов/записей в БД;
  реальные страницы30→60 без per-row fetch, search/calendar/quote,demo labels,
  обе темы/mobile390 без overflow/pageerror; снимки осмотрены.
- Национальный HTTP warm-cache пилот, PG17 2CPU/RAM limit1536MiB/shared_buffers256MiB:
  13 типов×24=312/312 запросов при concurrency4; warm-up без ошибок.
  p95: одна буква71.62ms,опечатка160.67ms,регион3.98ms,stop→stop45.46ms,
  city→city107.90ms,calendar109.82ms. EXPLAIN:26.78/23.22/30.68ms для
  destination suggest/city search/calendar, без Seq Scan trips/trip_stops.
  Это локальные измерения, не production SLA. Полные отчёты вне Git.
- GitHub CI опубликованных SHA PASS: backend `5a80b8c`,
  https://github.com/SaRgEX98/corpord-backend/actions/runs/41642332364
  (Go vet/race/PG/Redis, Python geometry + все 5 catalog tests);
  frontend `809c2aa`,
  https://github.com/SaRgEX98/corpord-frontend/actions/runs/41642406875
  (npm ci/typecheck/48 tests/production build). Publish image jobs пропущены
  по правилам PR workflow; образы в registry не публиковались.

Состояние стенда: сохранены только отдельные `corpord-catalog-test-pg`
(127.0.0.1:55439) и `corpord-catalog-test-redis`(127.0.0.1:56389), без production
данных/аккаунтов. Тестовая БД и отдельная custom-format резервная копия остаются
вне Git. E2E свои временные контейнеры/сети/аккаунты удалил. Локальный preview
на127.0.0.1:3104 и API8086 запущены процессами текущей сессии, их наличие нужно
проверять заново. Runtime overlay `corpord-backend:catalog-test` локальный,
не опубликован как production image. Другие контейнеры не менялись;
corpord-infra и реальные env не менялись.

Далее: ревью PR15/7 и решение о merge/целевом окружении; отдельная сверка полноты
официального перечня, unknown mappings/группировки station-platform, реального
расписания/тарифов, retention и бюджета нагрузочного/резервного контура.
Национальные Photon/PMTiles, бронирование/оплата и реальные продажи не включены.

## Продолжение: реальный геопилот, 2026-11-01

Этот раздел обновляет исторические ограничения ниже: настоящий городской Photon
и PMTiles уже проверены. Участник не разрешал merge/deploy; оба PR OPEN/ready.

| Репозиторий | Опубликованный HEAD feat/route-integration | PR |
| --- | --- | --- |
| Backend | `530970c` | https://github.com/SaRgEX98/corpord-backend/pull/14 |
| Frontend | `8388944` | https://github.com/SaRgEX98/corpord-frontend/pull/6 |

Integration worktrees чистые, изменения опубликованы. Origin dev повторно fetched:
backend `0d6180d`, frontend `2f3e1eb`; пользовательские изменения основного backend
сохранены. SSH credential недоступен в этой сессии; push/fetch выполнены через HTTPS
и существующий `gh auth git-credential`, без изменения сохранённых remote URL.

Геоданные вне Git: `corpord-geo-pilot-data/{photon,maps}`. Адреса: разрешённый
Geofabrik Урал 261031 (PBF 396673710 B, OSM timestamp 2026-10-31T21:23:06Z),
городской bbox `[60.45,56.72,60.75,56.92]`. JSONL 49919 кандидатов → Photon 49872
документа; 47 без имени/номера не индексируются. Подготовка 298.53s, maxRSS~1.02GiB,
наблюдённый временный диск~1.3GiB. Baseline import66.84s, наблюдённый cgroup
memory.peak~754MiB, без OOM; restart14.41s с сохранением реальных ID. Runtime
working set~446MiB — снимок, не нагрузочный предел. Датасеты/индексы не в PR.

Карта: реальный Protomaps PMTiles extract 20261031, 6462836 B, 360 MVT tiles,
zoom0–14; extract27.83s, maxRSS60900KiB. Свои стили light/dark, Noto Sans с
кириллицей, sprites, лицензии/ODbL attribution; все ресурсы локальные. Полный
воспроизводимый протокол: backend `deploy/geo/pilot/README.md`, photon.md,
map-README.md, quality-report.md, city-boundary.md. Источники/инструменты/hashes
закреплены. Это Екатеринбург, не покрытие всей России и не production SLA.

Реальные данные выявили отсутствующий production worker MapLibre6: исправлен
Vite `?url` + `setWorkerUrl`, ленивое подключение сохранено. Реальный выбор города
выявил SQL integer=text в `/locations/city:N/stops`: исправлены явные SQL casts,
добавлена Go PostgreSQL-регрессия для city/stop ID. Независимое ревью дополнительно
исправило pilot CORS для стандартного E2E порта3101 и проверку ошибок карты после
смены темы. Снимки real-map-light/dark/mobile осмотрены: дороги/здания/русские
подписи/атрибуция видимы; скриншоты в frontend `test-results`, вне Git.

Качество raw Photon: те же 40 source-grounded запросов ×3, 120/120 HTTP,
структурных ошибок0. Полные имена/адреса19/19, префиксы5/8, опечатки5/6,
неоднозначные2/3; вне покрытия/бессмысленные4/4 пустые. Baseline p50/p95=67/181ms.
Город есть только у11371 кандидата22.8%. Отдельная immutable city-enrichment-v1
копия добавляет city13957 узлам внутри **точной** OSM city relation6564910,
с отступом100м от всех границ; 83 ways/6816 nodes/1polygon/23holes, PBF и geometry
SHA pinned, provenance каждого заполнения. Явные города/ways с приблизительным
центром не меняются. City теперь у25328 (50.7%), неизвестное сохраняется.
Импорт копии59.16s без OOM; новый пик RAM не измерялся. Те же40x3: результаты
сохранены, 120/120 HTTP,0 structural; p50/p95=93.9/293.9ms. Свежий baseline control
98.6/214.8ms; это последовательные прогоны, причинный вывод о скорости запрещён.
City поля всех ответов проверены против соответствующего источника.

Проверки актуального кода:
- Go полный `GOMAXPROCS=2 go test -p2 -race -count=1 ./...` с отдельными PG17/Redis7
  + `go vet -p2 ./...` PASS; geometry guards5/5 PASS, включены в GitHub CI.
- Node26.1 typecheck,45/45 unit/contract, production build PASS. Node26 находится
  вне Git: `corpord-geo-pilot-data/tools/node-v26.1.0-linux-x64/bin`; host Node20
  не подходит текущему npm/проекту, перед Node-командами установить PATH.
- Финальный полный production Chromium с реальным Photon city-enrichment-v1,
  собственными PMTiles/шрифтами, новым backend и отдельными PG/Redis:
  **6 PASS /1 intentional skip** прежнего искусственного стиля карты.
  Реальная подсказка → город → тестовая внутренняя остановка, отсутствие публичных
  запросов, обе темы/mobile,9CRUD,цены/календарь/quote,права/logout-all.
  Map-only тест подменяет маршрутные API; geo integration подмен API не использует.
- Новый backend server локально собран как runtime overlay
  `corpord-backend:geo-integration-test` поверх прежнего тестового image, не
  полноценная Dockerfile-сборка и не публикация. Миграции те же, что в первом этапе.
- GitHub CI опубликованных SHA — PASS:
  backend `530970c`, https://github.com/SaRgEX98/corpord-backend/actions/runs/41292573247
  (Go vet/race/PG/Redis + Python3.14 geometry guards);
  frontend `8388944`, https://github.com/SaRgEX98/corpord-frontend/actions/runs/41292629742 .
  Publish/build job пропущен на PR по правилам workflow; образы не публиковались.

Все pilot Photon/nginx, их import-контейнеры и отдельные PG/Redis удалены;
временный файл credentials удалён, E2E удалил свои сети/БД. Данные оставлены для
повторного запуска. Действующие corpord frontend/backend/PG/Redis healthy,
не перезапускались задачей. Corpord-infra и реальные env не менялись.

Дальше: ревью PR14/6; до активации на окружении подтвердить целевые регионы,
город/timezone mapping остановок, смысл старых UTC дат, домены/CORS, бюджет
нагрузки/резервирования/обновлений. Связь геоместа с остановкой только явная.
Остатки unknown, резервы/платежи/надбавки спроса не реализованы до правил/источника.

## Первый этап маршрутов готов к PR-ревью: 2026-11-01

Пользователь поручил обновить `dev`, распределить задачи агентам и начать работу.
Три агента выполнили frontend, backend и geo-подзадачи в отдельных worktree;
корневой агент согласовал контракты, провёл интеграцию и независимые проверки.
PR #5 с планом смержен; оба новых PR открыты, ready for review, без merge/deploy.

| Репозиторий | Ветка / HEAD | PR в dev |
| --- | --- | --- |
| Backend | `feat/route-integration`, `36a73e6` | https://github.com/SaRgEX98/corpord-backend/pull/14 |
| Frontend | `feat/route-integration`, `6010142` | https://github.com/SaRgEX98/corpord-frontend/pull/6 |

Integration checkout: `corpord-backend-route-review` и
`corpord-frontend-route-review`, оба чистые и опубликованы. Рабочие ветки агентов
остались локально: backend `feat/route-search` (`cf543b0`), geo
`feat/self-hosted-geo` (`3ecb009`), frontend `feat/route-experience` (`5dcdaed`).
Основные `dev` обновлены до frontend `2f3e1eb`, backend `0d6180d`, но новые PR
в них не смержены. Основной backend остаётся на `local/seed-work-preserved` с
прежними пользовательскими seed/roles-изменениями, они не включены в эти PR.

Результат: внутренние города, явное сопоставление остановок и IANA timezone,
ранжированные подсказки, серверный поиск участков/пагинация, календарь/quote и
точные суммы копеек RUB. Миграция `20261101000100` сохраняет legacy NUMERIC,
добавляя generated bigint, города и mapping. Цена участка — сумма price_to_next,
без двойного сложения base_price; неполный тариф → null. Подтверждение политики
продаж/сборов ещё требуется. Timestamp трактуется как прежний UTC; перед выкладкой
проверить семантику реально импортированных дат и подтвердить mapping остановок.

Frontend: shadcn/Reka Select в общих/админ формах, русские дни/месяцы и фактический
понедельник в календаре, токены тем, центрированный AlertDialog logout/all с фокусом,
ошибкой/повтором, поиск и восстановление канонических точек из URL, quote в карточке,
внешнее место → явный выбор города/остановки, ленивые MapLibre/PMTiles.

Геостек: собственный Photon-адаптер (PHOTON_BASE_URL/PHOTON_TIMEOUT) с лимитами,
таймаутом и без публичного fallback. Версионируемые шаблоны `deploy/geo` backend:
закреплённые Photon/Java/nginx, Compose, Range/CORS/cache, импорт/обновление/rollback.
Frontend настройки NUXT_PUBLIC_MAP_STYLE_URL/NUXT_PUBLIC_MAP_DARK_STYLE_URL по
умолчанию пусты. Действующий corpord-infra и реальные env не менялись.

Актуальные проверки этих изменений:

- Backend полный `GOMAXPROCS=2 go test -p 2 -race -count=1 ./...` с отдельными
  PostgreSQL17/Redis7 и `go vet -p 2 ./...` — PASS; миграция down/up, копейки,
  участки/циклы, цена/null, календарь/quote, даты/пояса, курсоры, mapping.
- Backend GitHub CI PR #14 — PASS:
  https://github.com/SaRgEX98/corpord-backend/actions/runs/41242596994 .
- Frontend Node26.1: чистый npm ci, typecheck, 45/45 unit/contract, production build
  — PASS. GitHub CI финального frontend SHA — PASS:
  https://github.com/SaRgEX98/corpord-frontend/actions/runs/41243044498 .
- Chromium из production frontend с новым реальным backend и отдельными БД:
  4/4 PASS, 1 intentional skip карты без runtime style URL. Проверены девять legacy
  CRUD, единая цена участка в календаре/поиске/quote, America/Los_Angeles browser
  против Asia/Yekaterinburg origin, русский календарь, темы/mobile390, права,
  logout/all: центрирование, начальный/возвращаемый фокус, отмена, 503 и повтор.
- Отдельная MapLibre fixture 1/1 PASS: WebGL, только собственные локальные стили,
  ленивая загрузка, смена темы и устойчивость к перерисовке родителя. Это background
  style без настоящей геоподложки, не проверка PMTiles-архива/покрытия.
- Реальный закреплённый Photon импортировал три синтетических объекта и выполнил
  русские запросы; nginx smoke Range/CORS/ETag/cache, Compose config — PASS.
  Метрики tiny fixture не являются sizing региона; реального регионального пилота нет.

Независимое ревью исправило расхождение календаря/поиска для неполных тарифов,
неоднозначные циклы, завышенный trip_count, гонки URL/Combobox, жизненный цикл карты,
контраст дат. Общий E2E выявил обязательную проверку необязательного city_id —
исправлена и весь стенд повторно PASS. Desktop/mobile снимки осмотрены; подписи
дней сокращены против наложения, полные цены/статусы сохранены в aria/title.
Снимки находятся локально в frontend review `test-results/route-calendar-*.png`
и `route-detail-mobile.png`, не в Git.

Локальный Go1.27.3 server/migrate собран в отдельный runtime overlay
`corpord-backend:route-integration-test` поверх старого GHCR dev; image не публиковался,
не развёртывался как действующий dev. Все одноразовые контейнеры/БД/сети и файл
временных учётных данных удалены. Действующие frontend/backend/PostgreSQL/Redis
повторно прочитаны: healthy, uptime не сбрасывался этой задачей.

Следующие этапы: ревью PR (backend прежде frontend), реальный разрешённый региональный
индекс/PMTiles/стили/кириллица и пилот качества/ресурсов до включения геосервисов,
mapping старых остановок, выбор достоверного источника мест и утверждение тарифной
политики. Места unknown; надбавки по спросу, бронирования, оплата и ML не реализованы
и не выдаются за работающие. Ревью/merge и deploy — отдельные шаги.
Документы: frontend `docs/ROUTE_DELIVERY.md`, `docs/ROUTE_FRONTEND_IMPLEMENTATION.md`,
backend `docs/ROUTE_SEARCH_API.md`, `docs/routes.openapi.json`,
`docs/adr/0001-route-money-time.md`, `deploy/geo/README.md`.

## Начата реализация плана маршрутов: 2026-11-01

Пользователь одобрил начало разработки и явно поручил обновить `dev` и раздать
задачи агентам. PR #5 проверен заново: MERGED, mergedAt 2026-10-31T21:02:07Z.
Оба `origin/dev` получены с GitHub. Локальный frontend `dev` fast-forward обновлён
до `2f3e1eb2d71c4a73fe7845ac1ce185d8b0ead2ab`; backend `dev` обновлён до
`0d6180d88781ee6f301b45c8f9b3e3343677da83`.

Пользовательские незакоммиченные seed/roles-изменения основного backend сохранены
на месте: checkout теперь `local/seed-work-preserved` на `b6b1e94`; содержимое файлов
не менялось. Ветка `dev` существует отдельно и соответствует `origin/dev`.
Не переключать грязный checkout на новый `dev` автоматически: пересекается user.go.

Работа поручена трём агентам в отдельных worktree/ветках:

| Агент | Worktree | Ветка / задача |
| --- | --- | --- |
| route_frontend | `corpord-frontend-routes` | `feat/route-experience`: shadcn, селекторы, AlertDialog, русский календарь, поиск/карта |
| route_backend | `corpord-backend-routes` | `feat/route-search`: города/timezone, подсказки, поиск участков, календарь/quote, точные цены |
| geo_infra | `corpord-backend-geo` | `feat/self-hosted-geo`: Photon-адаптер, тесты, версионируемые шаблоны запуска/подложки |

Корневой агент выполняет согласование контрактов, интеграцию и ревью в
`corpord-frontend-route-review` / `corpord-backend-route-review`, ветки
`feat/route-integration` обоих репозиториев. Реализационные PR опубликованы и проверены: актуальный итог выше.
Не считать работу агентов проверенной до отчёта о новых проверках.

Используются отдельные одноразовые PostgreSQL/Redis этой задачи; действующие
контейнеры прочитаны по статусу, но не менялись. Никакого deploy/merge новых
реализаций не разрешено этим шагом. Остатки мест и надбавки по спросу не выдумывать:
источник мест/политика ещё не определены; новые API должны честно возвращать unknown.

## План развития маршрутов, карт и календаря цен: 2026-11-01

По запросу пользователя подготовлено задание для внутренней команды разработки.
План опубликован для ревью в frontend PR #5:
https://github.com/SaRgEX98/corpord-frontend/pull/5 .
Ветка `docs/route-experience-plan`, commit `41c2b2a`, база `origin/dev` =
`ad5a23120137a2dbbb1e347d09441ef10e9fb5d5`. Документ:
`corpord-frontend-planning/docs/ROUTE_EXPERIENCE_PLAN.md` (отдельный worktree;
после merge будет `corpord-frontend/docs/ROUTE_EXPERIENCE_PLAN.md`).

Зафиксированы требования: продолжать shadcn-vue/Reka; использовать существующие
токены цветов во всех селекторах; центрировать подтверждение logout/all; русский
календарь с понедельника и датой по поясу отправления; подсказки городов/остановок/
мест, карта, календарь актуальных цен и доступности; возможность повышения цены
по спросу. Ревью будущих реализаций также через отдельные PR в `dev`.

Предложено ранжирование по правилам без ML: релевантность, наличие направления,
затем популярность; Москва получает начальный приоритет при обслуживании, но не
фиктивные рейсы. После обсуждения пользователь выбрал открытый стек для пилота:
OpenStreetMap + MapLibre GL JS + собственный Photon; города и остановки ищутся
в БД Corpord. Для собственной подложки предложен PMTiles. Яндекс/2ГИС/DaData
исключены из основного плана. Публичный Nominatim не использовать для autocomplete;
публичные тайлы OSM и демосервер Photon не являются production-зависимостью.
План дополнен проверками русских адресов/POI, замерами регионального индекса,
ресурсами импорта/обновления, раздачей подложки, атрибуцией и условиями лицензий.
Эксплуатационные расходы остаются; ресурсы и период обновления определить пилотом.
Официальная документация изучена; перед внедрением сверить актуальные требования.

Пользователь явно указал порядок: обновить план → он одобряет PR #5 → можно
приступать к реализации. Обновлённый документ и описание PR опубликованы; затем PR смержен, пользователь
явно разрешил начало разработки (актуальный раздел выше). Одобрение плана разрешает разработку; ревью и merge
реализационных PR остаются отдельными шагами. Автоматический merge/deploy не разрешён.

Порядок работ: UI-исправления → справочники и геоадаптер → серверный поиск участков,
денежная модель и календарь → frontend-интеграция → достоверный учёт мест → тарифы
по загрузке. План включает предлагаемый OpenAPI, критерии приёмки и PR-зависимости.
Вместимость автобуса не равна остатку мест; таблицы заказов не означают готовый API
продаж. Источник остатков, правила тарифов участков, TTL и пределы повышения остаются
открытыми решениями. До этого допустим календарь расписания с явным unknown мест.

Проверено заново: GitHub dev SHA frontend выше и backend
`0d6180d88781ee6f301b45c8f9b3e3343677da83`; локальный frontend `6957d3f`
на `feat/complete-frontend` был чистым; основной backend `b6b1e94` на `dev`
с пользовательскими seed/roles-изменениями не менялся. Документный PR содержит
только план. Проверка diff на пробельные ошибки выполнена; runtime-тесты, контейнеры
и БД в этой задаче не проверялись и не менялись. Функции из плана ещё не реализованы.

## Обновление из GHCR dev: 2026-11-01T00:36:48+05:00

По явному разрешению пользователя скачаны и развёрнуты обновлённые `dev` образы
через основной `corpord-infra/docker-compose.yaml`. Локальный seed override при
этом запуске не применялся. Текущий backend заменён с `seed-local` на GHCR dev;
заполненные ранее данные сохранены. Предыдущие разделы о PR, ожидании merge и
работающем seed-local — исторические состояния.

| Сервис | Commit OCI revision | Образ | Результат |
| --- | --- | --- | --- |
| Backend | `0d6180d88781ee6f301b45c8f9b3e3343677da83` | `ghcr.io/sargex98/corpord-backend:dev` | healthy |
| Frontend | `ad5a23120137a2dbbb1e347d09441ef10e9fb5d5` | `ghcr.io/sargex98/corpord-frontend:dev` | healthy |
| Migrate | backend revision выше | backend dev | exit 0, новых миграций нет |

PostgreSQL и Redis также healthy. Версия Goose `20261031000100`.
Успешные dev workflow перед pull: backend
https://github.com/SaRgEX98/corpord-backend/actions/runs/41234568743 , frontend
https://github.com/SaRgEX98/corpord-frontend/actions/runs/41234691832 .
Revision скачанных образов и запущенных контейнеров сверены с актуальными GitHub dev.

Перед обновлением остановлены приложения и сохранён/проверен PostgreSQL custom
archive `corpord-infra/backups/before-frontend-dev-20261101-003540.dump` (71796 bytes, mode 0600). Содержимое
архива и секреты окружения в отчёты не выводились. Сравнение до/после: 4 пользователя,
6 автобусов, 10 водителей, 14 остановок, 56 рейсов и 224 остановки рейсов — без изменений.

Проверки после обновления: статусы/revision обоих приложений и migrate, 6 публичных
API коллекций, runtime адрес API frontend, PNG логотип и сохранность количеств —
15/15 PASS. В настоящем Chromium проверены главная, каталоги и карточки рейса/
автобуса/водителя/остановки, тёмная тема после reload, mobile 390px и отсутствие
ошибок JS — PASS. Авторизация существующими личными/демо-аккаунтами в этой проверке
не выполнялась. Новые тестовые аккаунты в действующей БД не создавались.

Адрес приложения: http://localhost:3000 ; API: http://localhost:8080/api/v1 .
Исходники backend с локальной seed-реализацией и файл её паролей не изменялись;
команда seed остаётся в отдельном локальном образе, в GHCR dev она не публиковалась.

## Новая frontend-реализация: PR на ревью владельца (2026-11-01)

Пользователь запросил реализацию всех backend endpoint во frontend, новый дизайн,
логотип и темы, план архитектуры, распределение между агентами в отдельных ветках,
независимое ревью и PR в `dev`. На момент создания PR работа была реализована и опубликована для ревью;
после разрешения владельца GHCR dev развёрнут — актуальное состояние указано выше. Разделы ниже про прежний
merge/deploy являются историческими результатами 2026-10-31.

| Репозиторий | Ветка и commit | PR в dev |
| --- | --- | --- |
| Frontend | `feat/complete-frontend`, `6957d3ff5a729dec6f16ae81744d5a594b5c74b2` | https://github.com/SaRgEX98/corpord-frontend/pull/4 |
| Backend (минимальное исправление роли) | `fix/frontend-role-contract`, `3c8d93f` | https://github.com/SaRgEX98/corpord-backend/pull/13 |

Ветки агентов опубликованы отдельно: `feat/frontend-api` (`f04546a`),
`feat/frontend-admin` (`cbb63cc`), `feat/frontend-design` (`38b1b5f`). Для них оставлены
worktree `corpord-frontend-api`, `corpord-frontend-admin`, `corpord-frontend-design`;
backend-исправление находится в отдельном `corpord-backend-role`. Основной frontend
worktree сейчас на `feat/complete-frontend`, чистый; backend-role также чистый.

Результат: типизированные сервисы всех 58 Swagger-операций (56 JSON и две generic
OAuth операции redirect/callback; legacy users aliases включены), реальные публичные
каталоги/карточки, административный CRUD 9 ресурсов и справочников, актуальный
профиль и управление сессиями. Новый адаптивный дизайн, AI логотип
`corpord-frontend/public/brand/corpord-mark.png`, light/dark/system с сохранением и
реакцией на ОС. Access token только в памяти. Добавлены очередь cookie transitions,
защита от запоздалых ответов и повторов старых mutation под новой identity.
В API нет бронирования/заказов, self-edit профиля и управления ролями; заглушки
не изображают такие действия как работающие.

**Порядок merge:** сначала backend PR #13 и публикация нового backend `:dev`, затем
frontend PR #4 и публикация frontend `:dev`. Старый GHCR backend на b6b1e94 отдаёт
пустую `role` в `/users/me` и admin user responses: независимый auth repository
авторизует admin, но профиль не читал роли. Исправление добавляет чтение persisted
role (user/admin/moderator) без миграций, ACL/seed изменений. Только точный admin
открывает админку. PR workflow frontend проверяет типы, тесты и production build;
из PR образы не публикуются. После merge действуют существующие GHCR dev/sha теги;
контейнеры обновляются отдельным pull/up после успешной публикации.

Проверено для указанных изменений:

- Node 26.1.0: чистый `npm ci`, typecheck, production build — PASS; 40/40 unit,
  contract и session tests — PASS.
- Playwright на production frontend с отдельными API/PostgreSQL/Redis: 2/2 PASS.
  Полный create/detail/update/readback/delete всех 9 ресурсов, relation ID, даты,
  нули, публичные каталоги/карточки, reload сессии, регистрация, logout/all, обычный
  user получает 403, темы и mobile 390px. Build-time API намеренно нерабочий:
  успешный CRUD проверяет runtime override NUXT_PUBLIC_API_BASE для готового образа.
- E2E harness дополнительно проверен на занятый порт (без обращения к действующему
  приложению) и SIGTERM: одноразовые контейнеры/сеть/temp env удаляются.
- Backend: полный `GOMAXPROCS=2 go test -p 2 -race -count=1 ./...` с отдельными
  PostgreSQL/Redis и `go vet -p 2 ./...` — PASS. Новый regression проверяет роли
  Create/GetByID/GetAll/GetByEmail/Update. GitHub CI PR #13 — PASS:
  https://github.com/SaRgEX98/corpord-backend/actions/runs/41233750221 .
- GitHub CI frontend PR #4 — PASS (typecheck, 40 tests, production build):
  https://github.com/SaRgEX98/corpord-frontend/actions/runs/41234069561 .
- Независимое перекрёстное ревью API/admin/design/backend/harness; найденные P1/P2
  исправлены. Browser integration выявил автоматическую числовую модель Vue input;
  нормализация исправлена и покрыта регрессией.
- npm audit после обновления lockfile: 0 critical/high, остаются 2 low/12 moderate.
  Nuxt 4.6.3, Vue 3.5.44, Router согласован с Nuxt 5.4.x, Zod исправленная 4.x.
  Удалены неиспользуемые Google Fonts и vee-validate Zod/Nuxt adapters. Есть
  upstream deprecation module.register() на Node 26; команды успешны.

Настоящий интерактивный SSO Google/Yandex в новых изменениях не выполнялся.
Локальный test image `corpord-backend:frontend-role-test` содержит бинарник ветки
3c8d93f (локальный Go 1.27.3, CGO off) поверх прежнего GHCR runtime. Локально нет
Buildx, полный исходный Dockerfile backend не пересобирался; Go версии go.mod
проверен GitHub CI. Test image не является публикацией GHCR dev. Все созданные в
этой задаче временные контейнеры/БД/сети удалены; реальное Compose не менялось этой
задачей.

Во время работы в основном backend-worktree появились/сохранились пользовательские
незакоммиченные изменения seed/roles: Dockerfile, Makefile, user.go, SEEDING.md,
cmd/seed, internal/seed, user_roles_test.go. Они **не включены** в наши PR и не
переписывались. На последней read-only проверке действующий backend использовал
пользовательский образ `corpord-backend:seed-local` (healthy), frontend — прежний
GHCR dev (healthy). Прежние заметки о полном соответствии deployed образов dev
теперь исторические; не откатывать этот локальный backend автоматически.

Документация в frontend: `docs/IMPLEMENTATION_PLAN.md`, `docs/ARCHITECTURE.md`,
`docs/API_COVERAGE.md`, `docs/BRAND.md` (prompt и путь AI asset), `docs/REVIEW.md`.
Backend: `ROLE_CONTRACT_FIX.md`. Следующие шаги владельца: ревью обоих PR, merge
в указанном порядке, дождаться GHCR и отдельно обновить нужное окружение.

## Проект и расположение

Приложение для работы с автобусными перевозками: пользователи, автобусы, водители,
маршруты, рейсы и остановки; frontend содержит вход, регистрацию и личный кабинет.
Workspace: `/home/timofei/Projects/TechGroup/corpord`.

| Директория | Назначение | GitHub |
| --- | --- | --- |
| `corpord-backend` | Go API и миграции | https://github.com/SaRgEX98/corpord-backend |
| `corpord-frontend` | Nuxt frontend | https://github.com/SaRgEX98/corpord-frontend |
| `corpord-infra` | Docker Compose, конфигурация и документация запуска | Пока без Git-репозитория и remote |

Backend: Go 1.24.6, Gin, PostgreSQL 17, Redis 7, JWT, OAuth2 Google/Yandex,
Goose, Swagger. Точки входа: `cmd/server/main.go`, `cmd/migration/migration.go`.
Frontend: Nuxt 4, Vue 3, TypeScript, Pinia, Tailwind. Важные места:
`app/store/auth.ts`, `app/plugins/api.ts`, `app/utils/session-refresh.ts`,
`app/pages/auth/callback.vue`, `app/middleware/auth-callback.ts`.

## Демонстрационные данные: 2026-11-01

По запросу пользователя реализовано и выполнено заполнение локального backend.
Исходники backend находятся на `dev`, HEAD `b6b1e94`; новые изменения пока локальные,
без commit/push. Добавлены `cmd/seed`, `internal/seed`, `SEEDING.md`, цель `make seed`,
бинарник `/app/seed` в Dockerfile. Также исправлено чтение роли пользователя:
`GetByID/GetAll/GetByEmail` и ответы Create/Update теперь возвращают текущую роль.
Ранее GET `/users/me` возвращал пустую роль даже для администратора; это обнаружено
при проверке демо-аккаунта и закрыто отдельными PostgreSQL-регрессиями.

Набор содержит 2 аккаунта (admin/user), 6 автобусов (включая резерв и ремонт),
10 водителей (включая отпуск/болезнь), 14 остановок с адресами и координатами,
4 направления из Екатеринбурга: Нижний Тагил, Челябинск, Тюмень, Пермь.
Отдельных таблиц маршрутов/адресов нет: используются `trips`, `trip_stops`, `stops`.
Адреса, станции, контакты и транспорт вымышлены; координаты — примерные тестовые.
В локальную БД загружены 56 рейсов, 224 остановки рейсов и 56 записей истории статуса
на **2026-11-01 — 2026-11-07 UTC**. Прежние 2 пользователя сохранены без изменений;
всего в локальной БД после наполнения 4 пользователя.

Демо-логины: `admin@demo.corpord.test` и `passenger@demo.corpord.test`.
Случайные пароли находятся только в `corpord-backend/.env.seed.local`, права 0600;
файл исключён из Git и Docker context. Не копировать его содержимое в отчёты.
Seed хеширует пароли bcrypt, не создаёт сессии. Существующие пароли и ручные
изменения демозаписей сохраняются; новые значения env не сбрасывают старый пароль.
Служебная таблица `demo_seed_entities` хранит принадлежность записей набору.
Повторный запуск не дублирует данные; пересекающиеся даты добавляют только недостающее.
Команда транзакционна, параллельные запуски сериализуются. Совпадение email/номера
с чужой записью отменяет заполнение; удалённая отслеживаемая запись требует
восстановления либо новой тестовой БД. По умолчанию — завтра UTC и 7 дней (лимит 31).
Старые даты автоматически не удаляются, повторения в разные дни расширяют историю.

Перед заполнением сохранён и проверен архив PostgreSQL:
`corpord-infra/backups/before-demo-seed-20261031-235630.dump` (63654 байта, права 0600).
Новые миграции схемы приложения не потребовались; ownership-таблица создаётся
только явно запущенным seed. Ничего не добавлено в автоматический запуск API.

Backend локально собран Dockerfile на Go 1.24.6 и заменён на образ
`corpord-backend:seed-local`, image ID
`sha256:37bf0fe48434fd4a76e0ad3a56d2de134a33452487667c3ca12c6d49a5b9d60b`,
OCI revision `local-seed-20261101`. Для него добавлен необязательный
`corpord-infra/docker-compose.local.yaml` (backend/migrate). Для сохранения
локального варианта используйте **оба** `-f`; обычный Compose из основного файла
выбирает GHCR и может заменить backend прежним образом без команды seed/исправления роли:

```bash
cd corpord-infra
docker compose -f docker-compose.yaml -f docker-compose.local.yaml up -d --wait
docker compose -f docker-compose.yaml -f docker-compose.local.yaml run --rm --no-deps \
  --env-from-file ../corpord-backend/.env.seed.local \
  backend ./seed --start-date 2026-11-01 --days 7
```

`run --env-from-file` проверен с Compose 5.6.1. Подробности локального Go-запуска,
переменных БД, паролей и контейнерного запуска: `corpord-backend/SEEDING.md`.
Образы в GHCR в рамках этого запроса не публиковались. На контрольной проверке
backend, frontend, PostgreSQL и Redis healthy; исторический migrate с exit 0.
Frontend-контейнер остаётся `ghcr.io/sargex98/corpord-frontend:dev`.

Актуальные проверки новых изменений: полный
`GOMAXPROCS=2 go test -p 2 -race -count=1 ./...` с отдельными PostgreSQL/Redis и
`GOMAXPROCS=2 go vet -p 2 ./...` — PASS. Интеграции проверили данные/связи,
последовательность остановок, цены, отсутствие пересечений автобусов/водителей,
повторения, пересекающиеся диапазоны, UTC, сохранение ручных правок, откат коллизий
и 4 параллельных seed. CLI проверяет генерацию/повторение файла, env-приоритет,
права 0600, пароли со спецсимволами, безопасные ошибки. Docker build, отсутствие
env в финальном образе и оба варианта Compose config — PASS. Проверено работающее
API: количества 6 публичных коллекций, вход обоих аккаунтов, `/users/me` с ролью,
admin `/admin/users` = 200 и user = 403, logout. Запуск seed из нового образа с
`--env-from-file` повторно создал 0 записей. Одноразовые тестовые контейнеры удалены.

Во время работы исходники frontend изменялись параллельно: на контрольной проверке
ветка `feat/complete-frontend`, HEAD `721148b`, рабочее дерево с изменениями.
Эти изменения не входят в задачу наполнения backend и здесь не проверялись.
Не считать прежнее утверждение о чистом `dev` актуальным для этого frontend checkout.
API заказов backend пока не реализован; заказы/платежи в демонабор не включены.

## Выполненная работа и Git

Пользователь запросил аудит backend, исправления с ревью, перенос в `dev`,
публикацию образов через GitHub Actions и локальную проверку Docker Compose.
Работа выполнена; изменения закоммичены, смержены и отправлены в оба `dev`.
`main` в рамках этой работы не обновлялась.

| Репозиторий | Commit dev после merge | Commit исправлений |
| --- | --- | --- |
| Backend | `b6b1e941f9a63fb975a68265c57465ae8b0e3ae9` | `5730895` |
| Frontend | `d0c436cbe18bb43d3615e9c175571bbd986b96d6` | `a89a9b3` |

Историческая проверка после аудита: оба репозитория на `dev`, рабочие деревья чистые,
`push` вернул `Everything up-to-date`. Этот файл и `AGENTS.md` сохраняются локально
в корне workspace, вне этих Git-репозиториев.

Подробный аудит: `corpord-backend/BACKEND_AUDIT.md`.
Состав изменений, проверки и ограничения: `corpord-backend/REVIEW_NOTES.md`.
Результат развёртывания: `corpord-infra/DEPLOYMENT.md`.

Основные исправления:

- ACL управления пользователями, проверка актуальной роли и существования пользователя.
- Атомарная ротация refresh token, защита от replay/гонок, согласование с logout/all,
  уникальный hash refresh token; устранены panic при отсутствии пароля.
- Одноразовый SSO state в Redis с привязкой к браузеру/провайдеру, PKCE,
  проверка ответов OAuth, транзакционное создание user+identity.
- SQL и DTO рейсов, ID сущностей, порядок остановок, частичные обновления,
  валидация, корректные HTTP-коды и обработка ошибок.
- Валидация конфигурации/JWT, CORS allowlist, таймауты, DB pool,
  graceful shutdown, исключение секретов из recovery-логов.
- Frontend callback, восстановление сессии и один общий refresh для конкурентных
  запросов с повтором после 401.

## Docker и CI/CD

Запуск выполняется из `corpord-infra`. Основной Compose использует готовые GHCR-образы;
локальное переопределение описано выше. Последовательность: healthy PostgreSQL/Redis →
одноразовый `migrate` → backend → frontend. `migrate` с exit 0 — штатный результат.
На последней проверке все четыре постоянных сервиса были healthy.

- Frontend: http://localhost:3000
- API: http://localhost:8080/api/v1
- Swagger: http://localhost:8080/swagger/index.html
- Образы: `ghcr.io/sargex98/corpord-backend:dev`,
  `ghcr.io/sargex98/corpord-frontend:dev`.
- Теги выбираются `BACKEND_IMAGE_TAG` и `FRONTEND_IMAGE_TAG`; локально оба `dev`.
- `.github/workflows/workflow.yml` в каждом репозитории проверяет изменения и
  публикует образы: `dev` → `:dev`, `main` → `:latest`, также `sha-<полный commit>`.
  Публикация в GHCR сама по себе не обновляет локальные контейнеры.

Успешные workflow данного развёртывания:

- Backend: https://github.com/SaRgEX98/corpord-backend/actions/runs/41189189898
- Frontend: https://github.com/SaRgEX98/corpord-frontend/actions/runs/41188998007

Перед миграциями нужна свежая резервная копия. Обычное обновление после успешного CI:

```bash
cd /home/timofei/Projects/TechGroup/corpord/corpord-infra
docker compose pull backend frontend migrate
docker compose stop backend frontend
docker compose up -d --pull never --wait
docker compose ps -a
```

## Конфигурация и SSO

Compose подставляет значения из `corpord-infra/.env` и явно передаёт backend
переменные из `docker-compose.yaml`. Файл `corpord-backend/.env` этот Compose не читает.
Сверять следует примеры `.env.example` в backend и infra. Реальные значения секретов
остаются только в локальных файлах окружения.

Исходная ошибка Google: `Missing required parameter: client_id`. После исправлений
проверены непустой client_id и переходы к формам входа Google/Яндекса.
Полный интерактивный вход личными аккаунтами пока не выполнялся.

- Каждый провайдер включается явно: `SSO_GOOGLE_ENABLED=true`,
  `SSO_YANDEX_ENABLED=true`; нужны соответствующие CLIENT_ID, CLIENT_SECRET,
  REDIRECT_URL. Некорректная конфигурация включённого провайдера блокирует запуск.
- Callback URL: `http://localhost:8080/api/v1/auth/google/callback` и
  `http://localhost:8080/api/v1/auth/yandex/callback`; у провайдеров должны совпадать точно.
- Backend callback устанавливает HttpOnly refresh-cookie и делает 302 на
  `${FRONTEND_URL}/auth/callback`. Frontend вызывает POST `/auth/refresh`,
  затем GET `/users/me`. Access token хранится в памяти frontend.
- `NUXT_PUBLIC_API_BASE` — адрес API, доступный браузеру, с `/api/v1`;
  Docker-имя `backend` для него не подходит. `ALLOWED_ORIGINS` содержит точные origins.
- Для локального HTTP: `COOKIE_SECURE=false`, `COOKIE_DOMAIN` пуст,
  `COOKIE_SAME_SITE=lax`. Для внешнего HTTPS параметры пересматриваются под домены.
- Изменения env требуют пересоздания контейнера; `docker compose restart`
  не обновляет его окружение.
- JWT_SECRET минимум 32 байта. При SSO HTTP_WRITE_TIMEOUT должен быть больше 10s.

## Миграции и проверки

Применена миграция `20261031000100_unique_refresh_token_hash`. Старый backend с
прежней ротацией несовместим с новым unique index; откат кода и схемы согласуется.
Перед развёртыванием сохранены `corpord-infra/backups/before-dev-20261031-231059.dump`
и `corpord-infra/.env.backup.20261031-143030`. Это чувствительные локальные файлы,
они исключены из Git. Историческая миграция автобусов не запускается повторно Goose.

Исторические результаты для указанных выше commit:

- Backend: 54 функции тестов, PostgreSQL/Redis integration, race detector и go vet — PASS.
  Команды: `GOMAXPROCS=2 go test -p 2 -race -count=1 ./...` и
  `GOMAXPROCS=2 go vet -p 2 ./...`. Интеграции требуют отдельные
  TEST_DATABASE_URL, TEST_REDIS_ADDR и при необходимости TEST_REDIS_PASSWORD.
- Frontend: `npm ci`, `npm test` (2 теста), `npm run build` — PASS.
- Compose config, сборки, миграции и 30 проверок работающего развёртывания — PASS.
- В браузере проверены парольный вход, профиль, refresh после 401, callback,
  восстановление сессии при перезагрузке и logout. Тестовые аккаунты удалены.
- Swagger перегенерирован: 58 операций совпадали с маршрутизатором.

## Открытые вопросы

1. Выполнить полный вход через реальные Google/Yandex аккаунты.
2. Спроектировать явное связывание SSO-аккаунтов. Сейчас совпадение email даёт 409,
   автоматического объединения аккаунтов нет.
3. При необходимости немедленного отзыва access token после logout спроектировать
   session/family ID. Сейчас access действует до TTL.
4. Один провайдер поддерживает один начатый вход на браузер: новая вкладка заменяет
   state-cookie, прежний callback отклоняется.
5. Отдельно решить денежную модель (сейчас float64), soft delete и timezone рейсов.
6. Dependency review frontend выполнен в PR #4 (2026-11-01): 0 high/critical,
   остаются 2 low/12 moderate; отдельно разобрать transitive advisories.
7. При необходимости версионировать infra и эти локальные заметки: пока remote для
   них не выбран, в GitHub они не опубликованы.

## Доступ к GitHub из текущего окружения

SSH origin не работал из-за ключа/askpass; HTTPS через авторизованный `gh` работал.
Remote можно не менять, используя для fetch/push параметры:

```bash
GIT_TERMINAL_PROMPT=0 git \
  -c url.https://github.com/.insteadOf=git@github.com: \
  -c credential.helper= \
  -c 'credential.helper=!gh auth git-credential' \
  push -u origin dev
```

Не выводить учётные данные в логи. Доступ и авторизацию проверять заново при необходимости.
