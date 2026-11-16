# winwid

Виджет для рабочего стола Windows: часы, календарь и события.  
Написан на Go + Wails (WebView2), данные хранятся локально в SQLite.  
Не требует установки PostgreSQL, Docker или других сервисов.

## Возможности

- Прозрачный виджет без рамки, закреплённый на рабочем столе.
- Мини-календарь и список событий на выбранный день.
- Добавление и удаление событий.
- Запоминание позиции виджета между запусками.
- Локальная база данных SQLite в `%APPDATA%\winwid\winwid.db`.
- Логирование в `%APPDATA%\winwid\winwid.log` (ротация после 1 МБ).

## Требования для сборки

- Go 1.21+
- Wails CLI v2:  
  `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Node.js 18+
- NSIS (для установщика):  
  `winget install NSIS.NSIS`  
  добавить `C:\Program Files (x86)\NSIS` в `PATH`.

## Разработка

```bash
wails dev