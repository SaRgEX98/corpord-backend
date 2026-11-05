# Сабагент: качество справочника

Вертикальный MVP по ../CONTEXT.md и ../CONTRACTS.md. Backend corpord-backend-dev:
internal/catalogquality/**, internal/handler/catalog_quality.go,
миграция20261105000200_catalog_quality.sql, quality tests/docs. Frontend:
отдельные quality API/types/components, pages/admin/catalog-quality.vue и ссылка
в layouts/admin.vue. Общие wiring-файлы backend интегрирует root. Никаких
commit/push/deploy; не трогать booking/passenger файлы других агентов.

Очередь stops city_id IS NULL с keyset/search. Summary отдельным bounded SQL;
города через существующий cursor API, только явный выбор. Показывать координаты,
адрес/неизвестное и признак использования, не предлагать nearest city как истину.
Сохранение требует reason и version, stale409. Row locks stop+city; запрет при
любых trip_stops. Ограничить гонку вставки trip_stop с ручной сменой mapping.
При несовместимом явном timezone вернуть409. Не менять source_snapshot и не
переписывать существующие расписания. Audit actor из middleware, before/after,
timestamp в той же транзакции. Никаких пользовательских email/секретов в audit.

Страница admin protected, lazy страницы30/DOM<=200, поиск городов при фокусе,
форма причины/понятные409/клавиатура/темы/mobile. Сохранение обновляет очередь
и историю, без обхода всех stops. Тесты ACL, stale, used stop, transaction race,
sync search trigger и защита ручной правки importer; query-count без N+1.
Только отдельная TEST_DATABASE_URL и test accounts. Сообщи root wiring API рано.
