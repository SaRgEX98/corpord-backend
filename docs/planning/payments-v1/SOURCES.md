# ЮKassa: первичные источники

Проверено 2026-11-05. Перед реализацией сверить снова. Архитектура Corpord в
STATE_MACHINE.md — наше предложение, а не обещание атомарности API провайдера.

- [Тестирование](https://yookassa.ru/developers/payment-acceptance/testing-and-going-live/testing):
  нужны ID/ключ тестового магазина; объект платежа имеет test=true. Для MVP выбран
  тестовый bank_card и hosted redirect. Не считать API sandbox отдельным URL.
- [Процесс платежа](https://yookassa.ru/developers/payment-acceptance/getting-started/payment-process):
  capture=false даёт две стадии, waiting_for_capture предшествует capture/cancel.
  Статусы pending/waiting_for_capture/succeeded/canceled; срок авторизации из
  provider expires_at не равен десятиминутному deadline места. Cancel относится
  к waiting_for_capture, после succeeded нужна операция refund.
- [Формат взаимодействия](https://yookassa.ru/developers/using-api/interaction-format):
  provider header **Idempotence-Key**, до64 символов, окно24 часа. У Corpord header
  **Idempotency-Key** — другой. Внешний ключ генерировать/сохранять отдельно для
  каждой операции. Не повторять неоднозначный create после окна вслепую.
- [Входящие уведомления](https://yookassa.ru/developers/using-api/webhooks):
  успешный ACK именно HTTP200. Доставка может повторяться до24 часов. Уведомление
  требует проверки подлинности/актуального состояния. В этом проекте основанием
  business transition служит authenticated GET объекта и сверка test/merchant/
  amount/currency/local intent. Не выдумывать HMAC header, отсутствующий в API.
- [Справочник API](https://yookassa.ru/developers/api):
  перед кодом сверить схемы create/get/capture/cancel/refund, ограничения суммы,
  metadata и ошибок. Webhook URL/HTTPS/порты и IP ranges сверять по официальным
  правилам перед настройкой reverse proxy; не доверять X-Forwarded-For от клиента.

HTTP200 после durable inbox commit, затем retryable reconciliation. Если событие
не удалось надёжно сохранить, не ACK success. Дедупликация по provider object +
event/current verified state; не предполагать наличие глобального event UUID.
Сохранять минимальные whitelisted поля; raw provider objects могут содержать PII.
Без тестовых ключей допустимы локальные contract fixtures/fake provider, но это
не пройденный настоящий sandbox. Ни tunnel, ни webhook подписки, ни секреты
самостоятельно из этого плана не создавать.
