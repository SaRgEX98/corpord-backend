# ЮKassa test adapter

`payments.NewYooKassa(payments.Config{ShopID, SecretKey, Timeout})` реализует
`internal/payments/port.go`. Config не имеет endpoint/live override. Принимаются
только numeric shop ID и ключ с префиксом `test_`; это локальный safety gate,
а не доказательство типа магазина. Аутентифицированный объект дополнительно
обязан содержать test=true и ожидаемый recipient.account_id.
Без настоящих тестовых ключей sandbox не запускался.
[Официальное тестирование](https://yookassa.ru/developers/payment-acceptance/testing-and-going-live/testing).

Endpoint фиксирован: HTTPS api.yookassa.ru/v3, Basic Auth, redirects запрещены
даже внутри origin. Default timeout10s, максимум30s; context поддерживается,
response ограничен64KiB. Адаптер не пишет логи/БД, ошибки не содержат URL, body
или credentials; Config/YooKassa formatting редактирован. Нет автоматической
смены operation key или retry loop. Provider Idempotence-Key1..64 ASCII берётся
из durable outbox отдельно для create/capture/cancel/refund. После24h потерянный
create нельзя слепо повторять: recovery/review решает checkout.
[Формат и идемпотентность](https://yookassa.ru/developers/using-api/interaction-format).

Create всегда capture=false, bank_card, redirect, save_payment_method=false;
metadata.corpord_intent_id хранит opaque checkout intent. Сумма — integer RUB
копейки, десятичная строка без float:100..35000000 (1..350000RUB), что соответствует
текущим опубликованным card limits. Полный refund того же checkout, без partial.
[Банковские карты](https://yookassa.ru/developers/payment-acceptance/integration-scenarios/manual-integration/bank-card).

Payment facts проверяют ID/status/test/merchant/card/no saved method/metadata/
RUB/decimal syntax. Create/capture сверяют ожидаемую сумму; create также intent.
GetPayment не принимает local intent/amount, поэтому checkout **дополнительно**
сравнивает факты со своим order при каждом переходе. Confirmation URL допускает
только HTTPS yookassa.ru/yoomoney.ru и их поддомены, без userinfo/fragment/чужого
порта; это дополнительное ограничение приложения. Адаптер URL не открывает.
[Схемы API](https://yookassa.ru/developers/using-api/openapi-specification).

Перед capture/cancel выполняется authenticated GET. Только waiting_for_capture
ведёт к POST; другой статус возвращается как текущий факт, а не как подтверждение
выполнения запрошенной операции. Checkout проверяет статус. Pending нельзя
считать отменённым. Refund object не содержит merchant/test: create/get refund
проверяют связанный payment через GET и полную сумму. Refund metadata при create
сверяется с intent. Решения seat/capture/compensation принадлежат checkout.
[Платёжный процесс](https://yookassa.ru/developers/payment-acceptance/getting-started/payment-process).

POST transport/timeout/408/409/5xx → Unknown: результат денежных действий не
известен. GET transport/5xx → Unavailable.429 → RateLimited с bounded RetryAfter,
401/403 → Configuration, identity/schema facts mismatch → Security.400/404/422
не означают, что ранее принятый запрос не выполнился; оркестратор сохраняет
неопределённость прежней попытки. Security/Configuration не разрешают освобождение
места, если ранее был capture intent.

`go test -race ./internal/payments` использует только локальный HTTP fake;
проверяет wire contract, stable duplicate effects, accepted-but-lost response,
timeout/HTTP errors, live/merchant/amount/intent mismatch, безопасные redirect,
размер body, fixed-decimal bounds и redaction. Никаких реальных ключей/платежей.
