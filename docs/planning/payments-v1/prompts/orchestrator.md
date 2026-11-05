# Root: интеграция checkout и кабинета

Прочитай ../CONTEXT.md, ../STATE_MACHINE.md, ../CONTRACTS.md, ../SOURCES.md и свежий
PROJECT_CONTEXT.md workspace. Используй project-code-index; полный архив не нужен.
Реализуй только явно порученный пользователем следующий этап, не запускай этот
план автоматически. Текущие dev baselines указаны в CONTEXT, сначала сверить origin.

Сначала изучи реальные booking locks/constraints/expiry, выбери один seat ledger
или безопасное расширение текущей таблицы. Зафиксируй миграционный план и точные
Go Provider port / DTO / состояния. До этого не поручай агентам независимо менять
одни и те же контракты. Модель из STATE_MACHINE — исходное предложение, проверь
все межпроцессные окна, cancel/expiry/departure и неизвестный результат capture.
Далее используй задания checkout-backend/provider-yookassa/passenger-frontend;
при необходимости работы агентов проверь, что пользователь поручил делегирование.
На лимите4slots запускай не более3 implementation агентов, QA после завершения одного.

Твоя зона: internal/config, app, handler registration, scheduler, OpenAPI, Compose
examples, docs. Единственный schema owner — checkout backend. Provider агент пишет
только adapter/notification intake, frontend — по закреплённому DTO. Передавай
ограниченный индексный context, individual prompt и ownership, не весь репозиторий.
Агенты не commit/push/deploy; интеграция/промежуточные cross-review у root.

PAYMENTS_ENABLED=false default, только test shop, fail closed при live/config
mismatch. Не просить ключи в чате. До внешнего sandbox можно завершить fake-provider
и isolated PG/Redis проверки. Sandbox с настоящими тестовыми ключами и публичным
HTTPS webhook — отдельная настройка пользователя; её отсутствие явно отметить.
Не публиковать tunnel/настраивать магазин без поручения. Реальные env не менять.

Проверки: полная текущая regression + QA matrix; production build, Go race/vet,
fresh migrations/down strategy, старые/новые holds и import/guards, no N+1,
возобновление worker после crash/duplicate/webhook reorder. Финальный отчёт разделяет
fake-provider PASS, настоящий sandbox PASS/не запускался и deployment state.
Обнови PROJECT_CONTEXT.md и bounded index. Реальные деньги/ticket/legal rollout
не считать частью технического sandbox. Готовность — ни двойного занятия, ни
второго capture/refund, ни потерянного денежного unknown без durable recovery.
