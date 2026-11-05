# Агент: развитие кабинета и checkout UI

Прочитай ../CONTEXT.md и root-утверждённый ../CONTRACTS.md. Базовый кабинет уже
есть: не переписывай auth/session или passenger CRUD с нуля. Используй index для
поиска booking.api, usePrivatePages, session-client и me/orders/passengers/profile.
Твой frontend checkout/ветка задаются root; ветку не переключать, не commit/push.

Зона: checkout API/types, компоненты оплаты/заказа, me/orders и детали
me/orders/[id], payment-return, entry из hold, sidebar и соответствующие тесты.
Не меняй backend/schema/quality; общий auth core только по согласованию root.

Раздели в интерфейсе удержания и demo-заказы; добавь фильтры по закреплённому API,
детали, цену/имя/участок/часовые пояса, понятные pending/unknown/paid/cancel/refund
и доступные server actions. Укажи тестовую оплату; demo подтверждение не настоящий
билет. Профиль/настройки использовать как есть, не добавлять выдуманный API.
Карта/паспорт/СНИЛС/платёжный метод не сохраняются. Имя пассажира — только шаблон.

Checkout POST — стабильный idempotency key для одного намерения; lost response→
повтор того же, не создание новой попытки. Confirmation URL только из backend,
не из query/user input, проверка HTTPS; redirect/return query не подтверждает paid.
На payment-return загрузить server detail, восстановить auth с безопасным local
redirect, показать статус даже после reload, закрытой provider вкладки/двух вкладок.
Polling только открытого pending detail, bounded/backoff по server hints, остановка
при terminal/unmount/hidden tab/logout; не reconcile каждого item при рендере.

Lazy30/max100, DOM200, debounce filters, abort+generation+sessionVersion для чтений
и мутаций. Не сохранять confirmation URL/PII в localStorage и очищать при смене
аккаунта. Локальный countdown не источник payment truth. Paid/capture-unknown
нельзя отменить через старый hold cancel; использовать server actions/error409.
Проверки: unit+typecheck+build, реальный isolated API/fake provider E2E; return до
webhook, после webhook, failed/cancelled/expired/retry/unknown, idempotent doubleclick,
чужой ID, account switch, mobile390/dark/keyboard, network loss. Никаких UI mock
успехов как доказательства настоящей оплаты. Root передать изменения/проверки.
