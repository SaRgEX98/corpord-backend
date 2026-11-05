# Предлагаемый протокол: сначала место, затем capture

Архитектурное предложение, подлежит фиксации root **до** параллельной реализации.
Заказ: `awaiting_payment → committing → paid`; до committing возможны
`cancel_pending → cancelled` и `expired`; ошибки сверки → `review_required`.
Компенсация успешного списания: `refund_pending → refunded` либо review_required.
Provider status хранить отдельно: pending/waiting_for_capture/succeeded/canceled.
Локальный timeout не означает provider canceled. Названия API согласовать с CONTRACTS.

1. Под user→trip/bus→allocation lock проверить свой действующий demo hold,
   snapshot/сумму/отправление; создать один order на hold и payment attempt/outbox.
   Unique intent и scope user+Idempotency-Key предотвращают дубли. Повтор не
   продлевает исходный hold. Commit, затем worker вызывает create payment.
2. ЮKassa create с `capture=false`, payment_method bank_card, hosted redirect.
   Сохранить отдельные постоянные provider keys для create/capture/cancel/refund.
   Потерянный ответ — unknown/retry/reconcile; не новый платёж с новым ключом.
3. Pending не продлевает TTL места. Пока клиент проходит оплату, истечение hold
   возможно. Поздняя авторизация не даёт права занять уже проданное место.
4. На проверенный через GET waiting_for_capture: транзакция под тем же trip/seat
   lock проверяет действующее allocation, исходный deadline, отсутствие cancel и
   future departure. Атомарно переводит allocation в committing и пишет capture
   outbox. После commit обычный expiry больше не освобождает это место.
5. Worker отправляет capture вне транзакции. Проверенный succeeded делает order
   paid, allocation confirmed без TTL. Capture canceled освобождает allocation.
   При неизвестном результате место не освобождать по простому таймауту:
   reconciliation/backoff/alert/review. Поздний succeeded не должен встретить
   перепроданное место. До capture intent проверить актуальность departure;
   при невозможности оказать услугу после succeeded нужна компенсация/refund.
6. Если hold истёк/cancel выиграл до committing, capture не создавать. Pending
   provider payment может ещё авторизоваться: наблюдать до terminal, затем cancel
   waiting_for_capture. API cancel ЮKassa не считать отменой pending. Если всё же
   пришёл verified succeeded без корректного места — no fulfillment, durable
   compensation refund; повторный webhook не создаёт второй refund.
7. Пользовательская отмена до committing ставит cancel intent под тем же lock,
   исключает будущий capture. После committing/paid нельзя вызывать старый
   hold cancel/Expire для освобождения:409 checkout_in_progress/paid_order.
   Отмена платной поездки пользователем/условия возврата — отдельная функция.

Root должен выбрать физическую схему и записать решение: либо один allocation
ledger с миграцией существующих holds, либо расширение booking_holds/constraints.
В обоих случаях **один exclusion** для всех блокирующих seat states, единый
lock order, guards trips/timetable/bus, адаптированные Create/Availability/Cancel/
Expire и importer. DTO старых holds не должен молча выдавать committing/paid как
expired: версия/добавочные поля и согласованное обновление UI обязательны.

Миграция сохраняет существующие active holds и истории; down не должен удалять
платёжные данные молча. Сначала миграции additive/backfill/constraints, затем
переключение readers/writers без окна, допускающего двойное занятие. Старый backend
после schema/protocol change может стать несовместимым: описать stop-the-world
локальный rollout/rollback и запрет запуска старого worker.

Crash recovery: outbox/inbox persistent в PostgreSQL, bounded batches, SKIP LOCKED,
короткие leases с reclaim и fencing/version при записи ответа; сеть вне locks.
Проверить lease lost, два workers и crash между capture и локальным paid commit.
Уникальность provider id, attempt/order, operation key, effect/refund. DB clock для
TTL; graceful stop/cancel; конечный retry budget не превращает unknown в unpaid.
Конечное ручное review сохраняет занятие/денежную неопределённость видимыми.
