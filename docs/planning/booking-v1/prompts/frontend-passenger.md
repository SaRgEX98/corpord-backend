# Сабагент: интерфейс пассажира

Реализуй frontend MVP по ../CONTEXT.md и ../CONTRACTS.md в corpord-frontend на
текущей ветке. Зона: booking API/types/components, me/orders, me/passengers,
trips/[id].vue, Sidebar.vue, booking unit/E2E tests/docs. Admin layout/page/API
принадлежат quality агенту. Не commit/push/deploy; не менять app/plugins/session
или общий transport API без согласования. Используй $api и существующую auth.

Карточка выбранного участка: lazy availability только после перехода в детали,
вход для бронирования, passenger saved/manual name, hold с новым ключом на новый
intent и тем же Idempotency-Key при transport retry. Не повторять POST автоматически
с новым ключом. 409 объяснить человеку и обновить availability. Таймер по expires_at,
не показывать expired как active; переход/отмена/смена аккаунта не оставляют чужие
данные/таймеры. Нет paid/confirmed/bought/QR билета: это удержание на10 минут без
подключённой оплаты, demo явен. Не выдумывать места у real trips.

Кабинет: cursor история удержаний с деталями/отменой; CRUD сохранённых имён
пассажиров. Lazy страницы30/DOM<=200, abort/generation, нет N+1/полных списков.
Доступность клавиатурой, focus/errors, русские даты с timezone рейса, обе темы,
mobile390. Навигация кабинета доступна после входа, login return сохраняет участок.

Тесты API key/signal/cursor, expiry/retry/owner session reset. E2E через отдельный
stack, не реальные аккаунты. Node26 в контексте; typecheck/unit/build. Сообщай
root несогласованности API, не придумывай новые endpoint без синхронизации.
