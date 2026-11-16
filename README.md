# winwid

A desktop widget for Windows: clock, calendar, and events.  
Written in Go + Wails (WebView2), data is stored locally in SQLite.  
No need to install PostgreSQL, Docker, or any other services.

## Features

- Frameless transparent widget pinned to the desktop.
- Mini calendar and a list of events for the selected day.
- Add and delete events.
- Remembers the widget position between launches.
- Local SQLite database at `%APPDATA%\winwid\winwid.db`.
- Logging to `%APPDATA%\winwid\winwid.log` (rotated after 1 MB).

## Build requirements

- Go 1.21+
- Wails CLI v2:  
  `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Node.js 18+
- NSIS (for the installer):  
  `winget install NSIS.NSIS`  
  add `C:\Program Files (x86)\NSIS` to `PATH`.

## Development

```bash
wails dev 
```
