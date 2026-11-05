# Сабагент: backend бронирования

Реализуй backend MVP по ../CONTEXT.md и ../CONTRACTS.md. Рабочая копия
`corpord-backend-dev`, текущую ветку не переключай. Твоя зона: internal/booking,
internal/handler/booking.go, миграция20261105000100, booking tests/docs. Общие
handler.go/app.go/service.go и OpenAPI интегрирует root. Другой агент пишет quality
в отдельных файлах. Никаких commit/push/deploy/реальных аккаунтов.

Не переиспользуй незавершённые legacy orders как будто там работает checkout.
Сделай holds, saved passengers, inventory snapshot только по затронутым demo trips.
Проверь конкурентный последний seat и лимит5 holds per user при разных trips.
Один seat нельзя занять на пересекающихся участках; соседние переиспользуются.
Идемпотентность scoped owner; повтор не продлевает TTL и не создаёт новую бронь.
Часы БД, row locks с единым порядком; TTL учитывается независимо от cleanup.
Quote/fare/отношения читать согласованно в транзакции, не доверять client price;
учесть конкурентную правку trip_stops/статуса. Не N+1 для списков истории.

Проверь владение сохранённым пассажиром; удаление сохраняет snapshot брони.
Timeout/cancel, strict input, безопасные ошибки. Запуск тестов только с отдельной
TEST_DATABASE_URL. Добавь meaningful PG concurrency, expiration, ownership,
idempotency, repeated-stop ambiguity, fare snapshot, pagination tests.
Сообщи root точный wiring API и DB изменения рано. Заверши кодом и проверками,
перечнем файлов, known limitations; не меняй чужие файлы и PROJECT_CONTEXT.md.
