# Предлагаемый контракт checkout v1

**Draft, не реализован.** Root сначала закрепляет DTO/state machine, после этого
backend/frontend агенты работают по одному контракту. Base `/api/v1`, auth как в
существующем API. IDs/ownership из middleware, RFC3339 UTC, integer minor RUB,
проверенные денежные диапазоны без float и потери JS safe integer.

| Метод | Путь | Назначение |
| --- | --- | --- |
| POST | /me/checkouts | body `{hold_id}`, Idempotency-Key →201/200 Checkout; повтор immutable intent |
| GET | /me/checkouts | cursor/limit/status → `{items,next_cursor}`; одна сводная query |
| GET | /me/checkouts/:id | CheckoutDetail, без обращения к провайдеру |
| POST | /me/checkouts/:id/cancel | durable cancel intent до committing, идемпотентный ответ |
| POST | /me/checkouts/:id/reconcile | rate-limited постановка сверки →202; повтор не создаёт оплату |
| POST | /payments/yookassa/webhook | без user JWT; schema/body/rate limits + provider verification |

Checkout summary:
```
{id,hold_id,trip_id,status,payment_status,amount_minor,currency:"RUB",is_demo:true,
 test:true,passenger_name,boarding_name,alighting_name,departure_time,arrival_time,
 timezone,arrival_timezone,seat_number,created_at,updated_at,hold_expires_at,
 actions:{can_pay,can_cancel,can_reconcile,can_view_confirmation}}
```
Status enum — STATE_MACHINE.md. payment_status публичный normalized enum:
not_started/pending/authorized/processing/succeeded/canceled/unknown/refunding/refunded.
Unknown — не success/decline. `hold_expires_at` — deadline первичного hold,
не deadline confirmed allocation. Действия вычисляет сервер; UI не выводит их
из одного provider status или local timer.

Detail добавляет `{payment:{confirmation_url:string|null},next_poll_after_ms}`.
URL только из проверенного ответа провайдера, HTTPS, не в списках/logs/localStorage.
Не создавать payment/refund при GET или reconcile. Auth return route
`/me/orders/:id/payment-return` читает серверный detail; query status не доверять.
Ссылка return_url строится сервером из разрешённого frontend origin, не client input.
Если intent создан, но URL ещё нет, вернуть pending без новой попытки оплаты.
Не отдавать secret/provider raw payload/card fields/internal operation keys.

Errors `{error,code}`:400 invalid_request;401;404 чужое/неизвестное;
409 hold_expired/checkout_exists/idempotency_conflict/checkout_in_progress/
paid_order/not_payable;429 rate_limited;503 payments_disabled/provider_unavailable.
Существующее auth middleware может иметь legacy error envelope: учитывать в клиенте.
Список keyset default30/max100, cursor scoped owner+filters, DOM200;
никакого OFFSET/total count и per-row passenger/trip/payment fetch.

Внутренний Provider port (до реализации оформить точными Go types):
CreateAuthorization, GetPayment, Capture, CancelAuthorization, CreateRefund,
GetRefund; контексты/таймауты, decimal strings, opaque IDs. Errors отличают
unknown outcome от definitive rejection и authentication/config error.
Оркестратор checkout владеет бизнес-решением capture/cancel/refund; adapter не
принимает решения о месте, не пишет business tables и не вызывает booking сам.
Provider агент возвращает проверенные facts; root связывает inbox/outbox workers.

Конфигурация (новая, пока не существует): PAYMENTS_ENABLED=false по умолчанию,
PAYMENTS_MODE=test (production режим в этом этапе отвергать), YOOKASSA_SHOP_ID,
YOOKASSA_SECRET_KEY только server, PAYMENTS_RETURN_ORIGIN. Настоящие env не менять
по этим промптам; .env.example только placeholders. Тестовый магазин и его ключ
нужно предоставить отдельно через secret storage, не в Git/чат/context. Проверка
`test=true` в ответе — дополнительная защита, она не делает live credentials
безопасными. Не посылать запрос с live credentials ради проверки режима.

В dev/synthetic разрешён только sandbox checkout; реальные рейсы не bookable.
Сумма0 не отправляется провайдеру: отдельный явно спроектированный free flow или
not_payable. Диапазон суммы сверить по актуальному API. Сохранение имени пассажира
не означает согласие на хранение платёжного метода. Чек/налоги/live ticketing
требуют отдельного продуктового решения; не выдумывать реквизиты продавца.
