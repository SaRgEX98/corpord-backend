# Агент: независимая проверка checkout

Прочитай ../CONTEXT.md, ../STATE_MACHINE.md и закреплённый ../CONTRACTS.md.
Сначала read-only review, затем disposable PG/Redis/fake provider; никакой основной
dev-БД, реальных аккаунтов, ключей или настоящих списаний. Не commit/push/deploy.
Проверь реальную реализацию, а не совпадение тестов с текстом SQL/документацией.

Минимальная матрица:
- last seat: два users/разные и пересекающиеся участки; adjacent reuse;
- create checkout retries/две вкладки/потерянный ответ, replay после terminal;
- authorize до/после expiry, cancel vs authorize/capture, departure до capture;
- crash до/после внешнего create/capture/refund, до локального commit;
- два workers, lease reclaim и поздний ответ потерявшего lease worker;
- duplicate/reordered/forged webhook, неверный shop/test/amount/currency/ID;
- неизвестный результат capture: место не перепродано, GET списка не зовёт provider;
- succeeded без fulfilment: один durable refund, повтор/refund timeout/recovery;
- confirmed сохраняет exclusion и trip/timetable/capacity guards;
- существующие active holds, passengers snapshots, migrations/backfill/rollback;
- ownership404, cursor/filter scope, SQL count1/30/100, no per-row provider calls;
- frontend return до webhook, reload, logout/account switch, mobile/dark/keyboard;
- payment disabled/live credential mismatch, redacted logs, body/rate limits,
  HTTP200 ACK только после durable intake, неизвестный external outcome виден.

Root сообщать file:line, воспроизводимый сценарий и последствия; исправления чужих
файлов сначала координировать. Укажи отдельно fake-provider coverage и реальный
ЮKassa sandbox (PASS или не запускался). Подтверждение sandbox не является live
readiness: фискализация/условия перевозчика/настоящие билеты вне этого этапа.
