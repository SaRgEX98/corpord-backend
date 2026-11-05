# Результат этапа — 2026-11-05

Реализованы три вертикали: demo seat holds, кабинет пассажира и admin quality
mapping. Общий контракт — [CONTRACTS.md](CONTRACTS.md), ограничения —
[BOOKING_HOLDS.md](../../BOOKING_HOLDS.md) и [CATALOG_QUALITY.md](../../CATALOG_QUALITY.md).
Исходные задания сохранены в [prompts](prompts). Они описывают выполненное
разделение работы; перед новым заданием обновить ownership и состояние веток.

- Backend: `feat/booking-workspace`, baseline38f5f71, две новые миграции.
- Frontend: `feat/passenger-workspace`, baseline df949be; требует нового backend.
- Основной dev остаётся на предыдущих образах. Merge/deploy этого этапа не выполнен.
- Исходный dirty checkout `corpord-backend` не менялся.

## Проверки

Изолированные PostgreSQL17/Redis7, UUID schemas и синтетические аккаунты:
полный Go race, go vet, 6 Python catalog tests. После финального ревью отдельно
повторены PG race сценарии отмены/expiry/ownership/idempotency/capacity и CORS
preflight с Idempotency-Key. Frontend: 54 unit tests, typecheck, production build.
Общий browser/API isolated E2E: 8 passed, 4 skipped (national/geo/maps fixtures
не включены); охватывает booking/passenger retry/cancel/expiry/ownership, quality
mapping/audit/stale version, lazy lists, маршруты, CRUD и mobile/theme.

Code index: валидатор навыка и 9 behavioral/security tests PASS. Проверяются
incremental edit/rename/delete, HEAD/branch/config, sensitive exclusions, quoted/
unquoted/DSN redaction, line anchors, FTS Unicode/escaping, budgets и длинный token.
SQLite connections закрываются после каждого запроса. Индекс не служит точным
call graph и не гарантирует обнаружения всех возможных секретов в произвольном коде.

## Продолжение

Сначала backend migrations/image, затем согласованный frontend. Новые backend API
аддитивны. Legacy search/calendar/quote по-прежнему не обещают остаток перевозчика.
Inventory создаётся по запросу только для затронутых demo trips; не заполнять holds
для всех600000 рейсов. Есть лимит5 active holds/owner и максимум10 минут с отсечением
по departure; `expires_at` авторитетен. Worker ускоряет очистку, но не обеспечивает
корректность stock. Saved passenger хранит только имя. Продажа/оплата/билеты,
уведомления, автоматический retention истории и carrier inventory — будущие задачи.

Навык и инструкция установки/конфигурации: [INDEX_USAGE.md](../../../tools/project-index/INDEX_USAGE.md).
Корневой PROJECT_CONTEXT.md содержит актуальные локальные checkout/PR/deployment;
он вне Git workspace. Прежняя история доступна отдельно, не читать её целиком
для обычной задачи: [archive](../history/PROJECT_CONTEXT_before_booking_20261105.md).
