# Реализация checkout — 2026-11-05

Ветки backend feat/yookassa-checkout (baseline dev e053337), frontend
feat/passenger-checkout (baseline8310ed7). Код реализован, не merged/deployed.
Основной dev продолжает работать на booking/quality images862feaa/8310ed7.
Пользователь разрешил pull и делегирование; три implementation агента завершили
backend checkout, adapter и кабинет, root связал конфигурацию/API/workers/E2E.
Исходный dirty seed checkout, основные БД/аккаунты/env не менялись.

## Поведение

Один physical booking_holds ledger/exclusion для held/payment_committing/confirmed.
Capture только после committing; неизвестный результат сохраняет место. Durable
jobs/inbox, короткие SQL locks, network вне tx, leases/fencing, постоянные provider
operation keys, ограничение окна повторного mutating запроса. Поздняя авторизация
отменяется, неожиданный succeeded без fulfillment компенсируется refund.
Отменённый refund не возобновляется из-за старого payment webhook; нужен review.
История/checkouts keyset, snapshot, owner404, constant SQL count и no-store.

Адаптер ЮKassa поддерживает только тестовый магазин, hosted redirect/bank_card,
проверяет merchant/test/amount/intent и не следует redirect с Basic Auth.
Конфигурация отключена по умолчанию; production mode/live key отвергается.
Кабинет: вкладки удержаний/заказов, фильтр, детали/return, bounded visible polling,
устойчивый retry и server actions. Password login безопасно возвращает к заказу.

## Проверки

Go race полного дерева и go vet PASS; checkout17 PG-тестов (+подслучаи), booking9,
provider9 top-level (~80 случаев), config/webhook/handler/scheduler и legacy suite.
Checkout покрывает гонки/последствия expiry/cancel/departure, потерянный create/
capture/refund, lease takeover/stale writer, snapshot/identity mismatches, old holds
migration/backfill/down refusal, soft-deleted owner recovery, страницы1/30/100 и
невозможность вызвать provider из GET. Provider coverage89.3%.
Python catalog6 PASS. Frontend64 unit tests/typecheck/production build PASS.
Общий real API/production Chromium + compile-tagged fake-provider:10 passed,
4 skipped (отдельные national/geo/map fixtures не включены). Среди новых сценариев:
response loss/replay, login return, forged return query, verified paid/reload,
unknown capture→verified recovery с одним effect, failure/cancel/compensation,
ownership, list без per-row GET, mobile390/dark. Снимок осмотрен.

Результаты относятся к fake-provider и отдельным PG/Redis/аккаунтам. Настоящая
ЮKassa sandbox **не запускалась**: нет отдельно предоставленных test credentials,
HTTPS webhook подписки/tunnel не настраивались. Live платежи/фискализация/реальный
перевозочный билет/пользовательские возвраты/retention не входят в этот этап.

## Review и rollout

[Протокол](../../CHECKOUT_PROTOCOL.md), [эксплуатация](../../CHECKOUT_OPERATIONS.md),
[OpenAPI](../../checkout.openapi.json), [adapter](../../YOOKASSA_ADAPTER.md).
Новая миграция20261105000300 обязательна, даже если оплата disabled. Перед её
применением backup и остановка старых API/workers; пары backend/frontend согласованы.
Down отказывается удалять payment history. Не выкатывать новую реализацию/секреты
автоматически из этого документа. Следующий шаг — review PR и отдельная настройка
тестового магазина через server secret storage; секреты не пересылать в чат.
