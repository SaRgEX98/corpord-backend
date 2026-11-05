# Агент: ЮKassa test adapter и intake уведомлений

Прочитай ../CONTEXT.md, ../SOURCES.md и root-утверждённый Provider port в CONTRACTS.
Сверь API по официальным ссылкам. Твоя зона internal/payments/**, отдельный webhook
handler/tests и fixtures/docs. Не меняй booking/checkout/DB migrations/wiring;
нужные durable inbox поля согласуй с schema owner до реализации. Не commit/push.

Реализуй create authorization (capture=false, bank_card, redirect), GET payment,
capture, cancel authorization, create/get refund. Нет generic произвольного URL:
фиксированный HTTPS API host, no cross-origin redirects с Authorization, context
+timeouts, bounded response body. Shop ID/secret server-only; тестовый режим по
умолчанию отключён, live путь отсутствует. Реквизиты карты принимает страница
провайдера, не Corpord. Объект test=false/не тот merchant/amount/currency/intent
вызывает review/security error, не бизнес-переход.

Provider Idempotence-Key≤64 отличается от клиентского Idempotency-Key. Передавай
стабильный отдельный key каждой операции, входящий от durable outbox, без случайного
нового ключа при timeout/retry. Не трактуй HTTP500/timeout как definitive failure.
За пределами24h неоднозначный create требует reconciliation/review, не blind POST.
Amount decimal string из integer minor без float; проверяй сумму GET response.

Webhook — только hint: size/rate/shape limits, известные event types, minimal
whitelist, durable inbox commit до HTTP200. Не предполагай event UUID/HMAC signature.
GET provider object c Basic Auth подтверждает данные; replay/reorder/подмена
payment_id/merchant/amount/test не должны менять чужой заказ. Нельзя выполнять
произвольные event payload URLs/доверять X-Forwarded-For без trusted proxy.
Поддельный неизвестный ID не должен превращать ingress в неограниченный API proxy.

Adapter не решает бизнес capture/seat/expiry: возвращает проверенные facts root
orchestrator. Contract tests через local HTTP fake: потеря ответа после accepted,
409/429/5xx/timeout, malformed amount, reordered events, unknown, headers/secrets
не в logs. Настоящие тестовые ключи и webhook публичный HTTPS не создавать/не
искать в реальных env самостоятельно. Если sandbox не запускался — честно указать.
