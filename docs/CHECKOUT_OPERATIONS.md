# Эксплуатация тестового checkout

Этап2026-11-05 добавляет код ЮKassa test shop и кабинет заказов. Реальный sandbox
не запускался: тестовые ключи магазина не предоставлены. Основной dev пока остаётся
на booking/quality images862feaa/8310ed7; новые feature branches не развёрнуты.
Fake-provider E2E проверяет интеграцию приложения, но не подключение ЮKassa.

## Настройки

По умолчанию PAYMENTS_ENABLED=false. Checkout create/webhook disabled, прежние
удержания работают. При наличии отдельно предоставленных **тестовых** реквизитов:
PAYMENTS_ENABLED=true, PAYMENTS_MODE=test, YOOKASSA_SHOP_ID и YOOKASSA_SECRET_KEY,
PAYMENTS_RETURN_ORIGIN (точный origin frontend), PAYMENTS_TIMEOUT1s..15s/default10s.
Ключи хранятся только в server secret storage/.env, не в Git/браузере/логах/чате.
Адаптер поддерживает только test_ credentials, numeric shop ID и дополнительно
проверяет test=true/merchant по ответам. Не пытаться проверять live key запросом
создания платежа. Настоящие списания/перевозочные билеты не поддерживаются.

Hosted redirect + bank_card, две стадии. Сумма checkout100..35000000 minor RUB;
нулевые и неподдерживаемые тарифы отклоняются. Return страницы не доказывает paid.
Webhook публичный HTTPS на /api/v1/payments/yookassa/webhook настраивается отдельно
пользователем в тестовом магазине. Не публиковать tunnel автоматически.
Подписки: payment.waiting_for_capture, payment.succeeded, payment.canceled,
refund.succeeded. Правила HTTPS/портов/прокси сверить в официальной документации.
Raw webhook не доказательство оплаты: workers запрашивают актуальные facts.

## Миграция и обновление

Новая20261105000300_checkout обязательна даже при отключённой оплате. Перед
обновлением резервная копия; остановить **все** старые API и workers. Миграция
расширяет существующий booking_holds ledger, единый GiST exclusion защищает held,
payment_committing, confirmed. Snapshot/старые активные holds сохраняются.
Поднять новую согласованную пару backend/frontend; все server instances должны
понимать новые states/guards/expiry. Не запускать старый backend рядом с новым.

Down отказывается при наличии payment/order/inbox history или защищённых seat
states. Не удалять историю вручную, чтобы обойти guard. Откат требует отдельного
плана согласования финансовых исходов/архивации либо восстановления проверенного
backup; обычная замена image на старый несовместима с расширенным протоколом.
Основной Compose использовать вместе с docker-compose.geo-dev.yaml.

## Восстановление и наблюдение

Provider calls выполняются вне DB tx/locks. Worker poll2s, serial runs, batch25,
execution budget10s; provider operation keys постоянные, lease30s + fencing.
Pending сверяется с задержкой; нормальный polling не исчерпывает failure budget.
20 последовательных неудач или security/config mismatch переводят заказ в
review_required. Неизвестный результат денег не означает canceled/unpaid.
Повтор mutating операции с неизвестным результатом ограничен23h от создания job,
внутри provider idempotence window; после этого automatic blind POST запрещён.

payment_committing блокирует seat независимо от TTL до verified outcome.
Поздний succeeded без действующего fulfillment запускает один компенсационный
refund. Неопределённый refund сохраняет защиту/видимый статус до сверки/review.
Не освобождать seat SQL-командой по таймеру, не выполнять новое списание вручную
с новым ключом, не переписывать payment statuses по клиентскому return query.
При review сверять provider ID/operation с журналом магазина через штатный доступ,
не публикуя PII/URL/ключи. Автоматического решения спорной оплаты нет.

PAYMENTS_ENABLED=false останавливает и исходящие workers: отключение конфигурации
не отменяет уже созданные платежи и не доказывает возврат. Перед выключением
разобрать pending/committing/refund/review, иначе recovery потребует повторного
включения тестового провайдера с тем же магазином/операциями. Allocation остаётся
защищённым. Retention order/inbox history пока не реализован; контролировать размер
таблиц и review_required/возраст pending/неистекающих committing.
Unknown webhook object IDs не вызывают provider GET и не сохраняются в inbox;
потерянный create response восстанавливается своим durable create job/ключом.
Известные события ACK200 после durable handling; ошибки intake дают503/429.
GET lists/details не обращаются к провайдеру; приватные responses no-store.

## Изолированная проверка

Собрать отдельный binary с -tags checkout_e2e ./cmd/checkout-e2e. Его управляемый
fake-provider и /__test/checkout/* находятся только в build-tagged harness;
cmd/server и обычные production images их не содержат. Не публиковать harness.
Для frontend isolated runner указать CORPORD_E2E_BACKEND_IMAGE с этим binary,
CORPORD_E2E_CHECKOUT_ENABLED=1 и свободные API/web ports. Runner создаёт/удаляет
свои PostgreSQL/Redis/accounts; реальные env и данные dev не читает.

Матрица fake faults: create/capture/refund response loss, duplicate/reordered
webhook, wrong amount/currency/test/merchant/identity, expiry/cancel/departure,
lease takeover/stale writer, no-spin polling, no N+1, private owner/filter cursor,
soft-deleted owner recovery, old holds/migration preservation/down refusal.
Настоящую банковскую sandbox проверку с ключами отчёт отмечает отдельно.

Источники и исходный контракт: docs/planning/payments-v1/SOURCES.md и CONTRACTS.md.
Точный внешний API: [checkout.openapi.json](checkout.openapi.json),
адаптер: [YOOKASSA_ADAPTER.md](YOOKASSA_ADAPTER.md).
