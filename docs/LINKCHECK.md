# Запуск проверки ссылок (check-links)

Короткая инструкция, как вручную запустить workflow проверки ссылок `check-links.yml`.

## Через веб-интерфейс GitHub
1. Откройте репозиторий (форк или upstream) в GitHub.
2. Перейдите в раздел **Actions** → найдите `check-links.yml`.
3. Нажмите **Run workflow** (обычно справа) и в выпадающем списке выберите ветку `add/downloaded-homepage`.
4. Нажмите **Run workflow** — статус и логи выполнения появятся в разделе Actions.

> Примечание: если вы запустите workflow в upstream (github/github-mcp-server), убедитесь, что у вас есть права на запуск workflow в этом репозитории.

## Через `gh` CLI
Если вы предпочитаете CLI и у вас есть локальная настройка `gh` с нужными правами, выполните:

```bash
gh workflow run check-links.yml --ref add/downloaded-homepage --repo <owner>/<repo>
```

- Замените `<owner>/<repo>` на `KARTYOM3248/github-mcp-server` (ваш форк) или на `github/github-mcp-server` (upstream), в зависимости от того, где хотите запустить.
- Для программы потребуется токен с правом `workflow` (и `repo` если репозиторий приватный). Убедитесь, что `gh auth status` показывает корректную учётную запись.

## Что делать при ошибке 403 при dispatch
- Ошибка `403 Resource not accessible by integration` означает, что текущий токен/интеграция не имеет права диспатчить workflow. Возможные решения:
  - Использовать личный токен (PAT) с скоупами `repo` + `workflow` и авторизовать `gh` или вызвать GitHub API с этим PAT.
  - Запустить workflow вручную через веб-интерфейс (Actions → Run workflow).

## Контекст
- Workflow: `.github/workflows/check-links.yml`
- Ветка по умолчанию для инструкций: `add/downloaded-homepage`

Если хотите, могу: (а) закоммитить и запушить этот файл в ветку `add/downloaded-homepage`, или (б) сразу выполнить `gh workflow run` при наличии PAT. Напишите, что предпочитаете.