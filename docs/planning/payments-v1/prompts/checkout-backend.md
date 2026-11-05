# Агент: checkout, заказ и занятость участка

Прочитай ../CONTEXT.md, ../STATE_MACHINE.md и закреплённый root ../CONTRACTS.md.
Перед кодом получи согласованное решение seat schema и Provider port; engineering
решения согласовывать с root, не останавливать пользователя из-за обычного выбора.
Работай только в назначенном checkout/ветке. Не переключай ветку и не commit/push.

Твоя зона: internal/checkout/**, checkout handler/tests, все новые SQL migrations,
интеграция с internal/booking. Root владеет app/config/router/scheduler/OpenAPI.
Provider adapter и frontend пишут другие агенты; чужие файлы не менять.

Order snapshot из своего live demo hold, unique hold/order и owner/key/payload.
Цена int64 minor, bounds и нулевые тарифы обработать явно. Согласованная занятость
held/committing/confirmed под одним exclusion; обнови allocation, availability,
expiry, cancel, trip/timetable/bus guards. Сохрани existing holds/history и
идемпотентные повторы; старый worker не должен освобождать оплаченные места.
Безопасность не должна зависеть от ежеминутного scheduler.

Нет сетевых вызовов под DB lock. Outbox записывается вместе с business transition,
платёжные keys сохраняются до первого запроса. Workers bounded/lease/fencing и
crash recovery. Capture разрешается только после durable committing; неизвестный
capture не освобождает место. Поздняя авторизация после expiry/cancel→cancel auth;
неожиданный succeeded без места→compensation refund, без ложного fulfillment.
После committing старый cancel не должен освободить участок. Не путай pending
cancel intent с поддержкой provider cancel только waiting_for_capture.

История/детали — owner predicates, keyset default30/max100 и constant SQL без
provider fetch/row. Снимки имени/timezone immutable; удаление пассажира не влияет.
Проверь на disposable PG гонки seat, checkout doubleclick, cancel/expiry/capture,
два workers, stale worker, lost response, confirmed соседние/пересекающиеся участки,
departure, guarded capacity/timetable, existing import и migration/backfill.
Передай root точные registration и worker signatures рано, затем результаты и
ограничения. Не использовать основную dev-БД и реальные аккаунты.
