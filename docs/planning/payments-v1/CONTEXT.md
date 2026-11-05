# Следующий этап: кабинет пассажира и ЮKassa sandbox

Дата: 2026-11-05. План реализован в feature branches; результаты — [STATUS.md](STATUS.md).
Готовность fake-provider не означает проверку настоящего магазина ЮKassa.
Пользователь выбрал ЮKassa, только тестовый магазин/тестовые операции.
Предыдущие PR backend16/frontend8 уже merged; рабочий baseline dev:
backend `862feaab573de141eaf0dd2916346bf9783e1f48`,
frontend `8310ed727a480a998cec71954ecec22ea2637e01`.
Состояние локального deployment проверять в корневом PROJECT_CONTEXT.md.

## Уже существует

- Backend Go/Gin/sqlx/pgx, PostgreSQL17/Redis7, auth/session/admin ACL.
- `internal/booking`: section holds, максимум5/user, до600 секунд с отсечением
  по departure, точные integer RUB копейки, имя/цена/время snapshot, idempotency.
- `booking_holds`: GiST exclusion по trip/seat/range **только status=held**.
  Expire/Create/Availability и trip/timetable/bus guards знают только held.
  Простое присваивание paid/confirmed немедленно ломает защиту от перепродажи.
- `app/pages/me/orders.vue`: история удержаний; `me/passengers.vue`: имена;
  `me/profile.vue`, `me/settings.vue`: существующий кабинет, проверить поведение.
  `booking.api.ts`, `usePrivatePages`, `useBookingClock`: уже есть lazy/cancel/session guards.
- Legacy orders/payments SQL и panic stub order service — не готовый checkout.
- Данные dev: реальные аккаунты плюс600000 synthetic trips. Для тестов только
  отдельные БД/Redis/аккаунты. Пользовательский dirty `corpord-backend` не трогать.

## Что добавить

1. Заказ из своего действующего hold, snapshot и однозначный checkout intent.
2. Hosted redirect ЮKassa: банковская карта тестового магазина, две стадии
   `capture=false`. Capture после надёжного закрепления места. PAN/CVC/сохранение
   карты, автоплатежи, СБП, групповые заказы и реальные списания вне этапа.
3. Оплаченные demo-заказы в кабинете: детали, статусы, понятное восстановление
   после return/reload/сетевой ошибки. Возврат на сайт не доказывает оплату.
4. Durable outbox/inbox, reconciliation, компенсация нежелательного успешного
   списания. Пользовательские частичные возвраты и настоящий перевозочный билет
   вне этапа. Подтверждение demo-заказа явно не является действительным билетом.

## Неизменные требования

Единое физическое ограничение занятости охватывает held, committing и confirmed
участки. Нельзя разделить их по двум таблицам с независимыми constraints.
Платёж и заказ — разные state machines. Не вызывать provider внутри DB lock/tx.
Цена/пассажир/seat берутся из snapshot сервера. Финальные состояния не откатываются
из-за повторного или старого webhook. Чужие ID404 независимо от cursor/body.
Списки default30/max100, scoped keyset, DOM200, постоянный SQL count; никаких
вызовов ЮKassa из GET списка/каждой карточки. Poll только текущего заказа и bounded.
Не собирать новые документы пассажира и не журналировать PII/URL оплаты/ключи.

## Порядок работы и владение

Задание root: [orchestrator](prompts/orchestrator.md). Сначала финализировать
[модель состояний](STATE_MACHINE.md) и [API](CONTRACTS.md), затем код параллельно.

| Агент | Владеет | Не меняет |
| --- | --- | --- |
| Checkout/seat backend | internal/checkout/**, booking allocation integration, все новые SQL migrations, checkout handler/tests | adapter ЮKassa, frontend, app/config wiring |
| Provider backend | internal/payments/**, webhook handler/tests, provider fixtures/docs | booking/checkout/schema/frontend/wiring |
| Passenger frontend | API/types checkout, me/orders/details/return, booking UI entry, sidebar/tests | backend/schema/auth core |
| Root | контракты, config/app/handler/scheduler wiring, OpenAPI, Compose examples, review/E2E | пользовательские seed-правки/реальные env |

QA prompt запускать после реализации отдельным проходом; максимум3 implementation
агента одновременно, учитывая root. При лимите4 slots: checkout+provider+frontend,
затем QA вместо завершившегося агента. План сам по себе не разрешает запуск агентов,
реальные платежи, создание публичного tunnel или deploy следующего этапа.
В новых заданиях явно определить ветки/checkout; агенты не переключают общие ветки,
не commit/push/deploy. Root интегрирует, проверяет и обновляет текущий контекст.

## Контекст без полного чтения дерева

Прочитать AGENTS.md и текущий PROJECT_CONTEXT.md. Использовать `$project-code-index`:
refresh → symbol Create/Cancel/Expire → bounded excerpts, затем rg для проверки.
Достаточный старт: этот файл, STATE_MACHINE, CONTRACTS и индивидуальный prompt.
Нужные исходники: `internal/booking/{service,passengers,model}.go`, текущая booking
миграция, `internal/scheduler/booking.go`, `app/api/booking.api.ts`, `me/orders.vue`,
`usePrivatePages.ts`, session-client.ts. Не читать весь исторический архив.
Официальные сведения/проверенные ограничения: [SOURCES.md](SOURCES.md).
Неизвестные настройки магазина/receipt/tax не выдумывать; live rollout вне этапа.
