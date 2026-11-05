# Corpord: бронирование, пассажир, качество справочника

Дата: 2026-11-05. Пользователь поручил реализацию, контекст, промпты сабагентам
и навык экономной навигации по файлам. Этот этап охватывает три рекомендованных
блока: удержание мест, кабинет пассажира, ручную проверку привязок остановок.

## Рабочие копии и ответственность

- Backend: `<workspace>/corpord-backend-dev`, ветка `feat/booking-workspace`, baseline
  `38f5f7148cf240a08ecabcd457ce1a1ebc619011`.
- Frontend: `<workspace>/corpord-frontend`, ветка `feat/passenger-workspace`, baseline
  `df949befb57cc32f24d177908f7d169b653577d2`.
- Исходный `corpord-backend` содержит пользовательские seed-правки: не менять.
- Root agent владеет wiring: `internal/handler/handler.go`, `internal/app/app.go`,
  `internal/service/service.go`, общим OpenAPI/интеграцией и PROJECT_CONTEXT.md.
- Backend агент: `internal/booking/**`, `internal/handler/booking.go`, миграция
  `20261105000100_booking_holds.sql`, backend booking tests/docs.
- Frontend агент: `app/api/booking.api.ts`, типы/компоненты booking, страницы
  `me/orders`, `me/passengers`, карточка trips/[id], sidebar и booking tests/docs.
- Quality агент: `internal/catalogquality/**`, `internal/handler/catalog_quality.go`,
  миграция `20261105000200_catalog_quality.sql`, quality API/тип/page, admin layout,
  quality tests/docs. Не менять файлы двух других агентов.
- Агенты работают в общих указанных checkout, не переключают ветки, не commit/push,
  не меняют общие wiring-файлы. Передают root точные функции регистрации и проверки.

## Проверенный baseline

Go/Gin/sqlx/pgx/PostgreSQL17/Redis7; Nuxt4/Vue3/TypeScript, Node26.1.
Слои handler → service/repository; маршрутный модуль `internal/routes` самостоятельный.
`s.Routes.Repo.DB` — sqlx DB. Auth middleware ставит integer `userID` в gin context,
роль проверяется существующим RoleMiddleware. JWT/refresh/account lifecycle готов.
`model.Order` отсутствует; `internal/service/order.go` — неиспользуемый panic stub,
старые orders/order_items/payments SQL существуют, но не являются готовым checkout.
Новый hold не притворяется оплаченным legacy order.

Геосправочник/поиск уже готов: source ownership, sync catalog_search, keyset API,
N+1 guards, lazy pages30, окно200, abort/generation guard. Эти свойства сохранять.
Публичные search/calendar/quote пока возвращают unknown availability. Наличие capacity
автобуса само по себе не означает доступность мест у реального перевозчика.
Сервис Quote считает точный участок в integer RUB копейках и проверяет дату/статус.
Связь остановка-город/timezone нельзя менять у используемых в расписании остановок.
Unknown mapping сохраняется NULL; ближайший город не является доказательством.

Локальный dev уже содержит 600000 demo trips и реальные аккаунты. Это **не** тестовый
стенд для новых мутаций. Использовать отдельные PostgreSQL/Redis/schema и аккаунты.
Не читать/печатать настоящие env/токены/пароли/дампы. Никаких merge/deploy в этом этапе.
Не считать исторический PASS свежим. Node26 лежит вне Git:
`<workspace>/corpord-geo-pilot-data/tools/node-v26.1.0-linux-x64/bin`.

## Решения MVP

- Одна бронь удерживает одно автоматически выбранное место на **участке** рейса.
  Смежные участки могут повторно использовать место; пересекающиеся — нет.
- Hold10 минут по часам PostgreSQL; статусы held/cancelled/expired. Подтверждения
  оплаты/продажи/QR-билета здесь нет. В UI назвать удержание и показать время истечения.
- Деморейсы: capacity берётся из автобуса, при первой записи фиксируется snapshot
  inventory; эффективная ёмкость min(snapshot, текущая bus.capacity). Уменьшение
  ёмкости/смена рейса при активном hold блокируются в БД. Это явно деморежим.
  Для real trips stock/bookable остаётся unknown/false.
  Не создавать inventory/holds заранее для всех600000 рейсов.
- Сохранённый пассажир содержит только имя; паспорт/дату рождения/платежи не собирать.
  Бронь хранит snapshot имени и цены, удаление шаблона не меняет историю брони.
- Владение проверять на каждом чтении/изменении; чужие ID возвращают404. Pagination
  всех новых списков: default30/max100, cursor привязан к owner/query, без OFFSET.
- Idempotency-Key обязателен при создании hold; тот же ключ/другой payload →409.
  Ограничить число активных удержаний пользователя5, включая гонки разных рейсов.
- Истечение учитывается при чтении stock/создании даже без фонового worker. Worker
  чистит ограниченные batch и корректно останавливается вместе с приложением.
- Качество: очередь unknown stops, поиск города, явное сопоставление с причиной,
  optimistic version и audit actor/before/after. Lock order city→stop совместим
  с importer; timetable writers берут trip→stop. Никакого nearest-city repair.
  Привязки остановок с любым trip_stops не менять; показывать понятный409.

## Готовность

Реальные PG-тесты гонок последнего места, пересекающихся/смежных участков,
expiration/idempotence/ownership; bounded list query count. Quality: stale version,
ACL, используемая остановка, importer защищает manual change. Frontend: реальные
API и отдельные аккаунты, timer/409/retry/login, сохранение пассажира, история/отмена,
quality mapping. Typecheck/unit/build/Go race/vet и миграции fresh/down-up.
Контракт — [CONTRACTS.md](CONTRACTS.md), задания — [prompts](prompts).
