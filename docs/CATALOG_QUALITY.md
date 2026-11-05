# Ручная проверка привязок остановок

Машиночитаемый контракт четырёх quality endpoints:
[booking.openapi.json](booking.openapi.json), OpenAPI3.0.3.

`/api/v1/admin/catalog-quality` доступен только администратору. Очередь содержит
только stops с неизвестным `city_id`, страницы default30/max100, курсор привязан к
поисковой строке. Одна страница — один SQL независимо от количества строк;
признак использования вычисляется EXISTS по индексу trip_stops. Сводка — отдельный
SQL aggregate с таймаутом3s. Это точное число, поэтому стоимость растёт с очередью;
сводка не выполняется для каждой страницы/строки. История ограничена20/max100.

Привязка требует явного ID города, актуальной `quality_version` и причины5..500
символов. Ближайший город автоматически не назначается. Транзакция держит
сначала `FOR SHARE` выбранного города, затем `FOR UPDATE` остановки, как импортёр
(city→stop). После блокировки проверяет существующие
trip_stops и совместимость явно заданного timezone. Используемая остановка
возвращает409 stop_in_use, устаревшая версия —409 stale_version, несовместимый
timezone —409 timezone_conflict. Никакого пересчёта расписаний/цен/часовых поясов.

BEFORE INSERT/UPDATE OF stop_id триггер trip_stops берёт SHARE lock остановки:
вставка расписания, начавшаяся раньше, заставляет mapping дождаться commit и
вернуть409; вставка после захвата mapping-lock ждёт завершения привязки. Триггер
версии увеличивает bigint при любом UPDATE stops. Все конфликты в Map откатывают
и изменение, и audit. Audit хранит actor ID из auth middleware, время PostgreSQL,
причину и before/after `{city_id,timezone}`; имён аккаунтов/email/токенов нет.
Timezone в audit — явное значение stops; в карточке — effective timezone города.

`source_snapshot` не переписывается. Импортёр сравнивает текущую привязку с
исходным снимком и защищает ручное изменение при повторном импорте; обычный
catalog_search синхронно обновляет city/region/timezone триггером текущего каталога.
Ручное редактирование существующим generic stop CRUD имеет прежнюю семантику;
новый контролируемый workflow и его аудит относятся к quality API. Глобальный
запрет старого CRUD — отдельное совместимое изменение для последующего этапа.

Frontend `/admin/catalog-quality`: lazy30, окно200, debounce250ms, AbortController
и generation guard. История грузится только выбранной остановки, варианты городов
только при фокусе. Ошибки409 показаны пользователю; stale требует обновления.

Проверка на отдельном TEST_DATABASE_URL, UUID schemas: Go race тесты optimistic
version/audit/search sync/import ownership predicate, SQL-count для1/30/100,
literal wildcard/cursor, обе стороны гонки trip_stops, down/up; handler ACL.
Отдельный конкурентный тест подтверждает порядок city→stop с импортоподобным
writer, реальным PG lock wait и stale409 после изменения источника. Неожиданный
deadlock/serialization/lock-unavailable возвращает безопасный503 quality_busy;
изменение и audit откатываются вместе. Старый SaveStop делает один UPDATE и не
берёт несовместимый city SHARE lock; FK KEY SHARE совместим с quality city SHARE.
Браузерный тест работает только на изолированном E2E stack и реальном API.
