# Индекс навигации Corpord

Канонический source навыка: [project-code-index/SKILL.md](project-code-index/SKILL.md).
Установленная копия: `~/.codex/skills/project-code-index`; навык автоматически
доступен для подходящих задач и вызывается явно как `$project-code-index`.
Конфигурация конкретного workspace — `.agents/project-index.json` в его корне.
Пример [workspace.example.json](workspace.example.json) предназначен для двух
репозиториев Corpord, а не для запуска из корня одного backend checkout.

Индекс учитывает backend-dev, основной frontend, выбранные infra-файлы и текущие
workspace-инструкции. Старые worktree и dirty основной backend не индексируются
как канонический исходник. Git tracked+nonignored untracked файлы учитываются;
binary/large entries имеют только метаданные. Секреты/дампы/геоданные/зависимости/
build outputs исключены. Полное содержимое binary не читается.

Из workspace:

```bash
python3 tools/project-index/project-code-index/scripts/code_index.py --root . refresh
python3 tools/project-index/project-code-index/scripts/code_index.py --root . symbol Expire --repo backend
python3 tools/project-index/project-code-index/scripts/code_index.py --root . context 'booking passenger' --max-chars 6000
python3 -m unittest discover -s tools/project-index/project-code-index/scripts -p 'test_*.py' -v
```

Локальный `tools/project-index` — alias канонической папки в backend. Навык можно
также запускать по абсолютному пути установленной копии. Перед обновлением копии
сравнивать source и installed; не перезаписывать чужой навык с тем же именем.

Cache SQLite FTS5 — `.agents/cache/project-index.sqlite3`, mode0600. Он одноразовый,
не хранится в Git. SHA содержимого обнаруживает правки даже с сохранёнными mtime/
размером; Git HEAD/branch/config и версия процессора инвалидируют stale состояние.
Переименование/удаление чистят выдачу. Это локальный lexical index, не semantic
embedding или точный call graph. `related` даёт кандидатов; проверять через rg.

При реальном handoff передавать query context и отдельные task/contract/ownership.
Ограничение6000 — символы, не точное число model tokens. Экономия зависит от задачи;
процент экономии не измерен. Индекс не отменяет чтение релевантного исходника.

Проверено9 behavioral/security тестов: изменения/перемещения/ветки/config,
отсечение реальных env и escaping symlink, redaction quoted/unquoted/URL credentials,
line anchors, Unicode/FTS escaping, bounded excerpts/context и длинный token50k.
В независимом прогоне найдено и исправлено2 дефекта redaction; контрпримеры включены.
