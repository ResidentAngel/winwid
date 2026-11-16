package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	_ "modernc.org/sqlite"
)

// schemaVersion хранится в PRAGMA user_version — по нему решаем, нужны ли миграции
const schemaVersion = 1

var errNoDB = errors.New("база данных недоступна")

type Note struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Time      string `json:"time"`
	Content   string `json:"content"`
	Completed bool   `json:"completed"`
}

type App struct {
	ctx        context.Context
	db         *sql.DB
	dbCloseOne sync.Once
}

func NewApp() *App { return &App{} }

// dataDir создаёт %APPDATA%\winwid и возвращает путь
func dataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "winwid")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// setupLogging у оконного приложения нет консоли, поэтому лог в файл
func setupLogging(dir string) {
	path := filepath.Join(dir, "winwid.log")
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC // ротация: больше 1 МБ — заново
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dir, err := dataDir()
	if err != nil {
		log.Printf("Не удалось создать папку данных: %v", err)
	} else {
		setupLogging(dir)
	}

	// путь можно переопределить переменной окружения WINWID_DB
	dbPath := os.Getenv("WINWID_DB")
	if dbPath == "" && dir != "" {
		dbPath = filepath.Join(dir, "winwid.db")
	}

	if dbPath == "" {
		log.Printf("Путь к базе не определён, данные сохраняться не будут")
	} else {
		db, err := openDB(ctx, dbPath)
		if err != nil {
			log.Printf("Ошибка открытия базы %s: %v", dbPath, err)
		} else {
			a.db = db
			log.Printf("База данных: %s", dbPath)
		}
	}

	go a.windowManagerLoop(ctx)
}

func (a *App) shutdown(_ context.Context) { a.closeDB() }

func (a *App) closeDB() {
	a.dbCloseOne.Do(func() {
		if a.db != nil {
			if err := a.db.Close(); err != nil {
				log.Printf("Ошибка закрытия базы: %v", err)
			}
		}
	})
}

// openDB настраивает соединение и накатывает миграции
func openDB(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// один писатель SQLite эффективнее когда всё идёт через одно соединение
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	for _, p := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := db.ExecContext(ctx, p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("миграция: %w", err)
	}
	return db, nil
}

// migrate создаёт таблицы, если база новая или старой версии
func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version >= schemaVersion {
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS notes (
			id        TEXT PRIMARY KEY,
			note_date TEXT NOT NULL,
			note_time TEXT NOT NULL,
			content   TEXT NOT NULL,
			completed INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_date_time ON notes(note_date, note_time)`,
		`CREATE TABLE IF NOT EXISTS widget_state (
			id    INTEGER PRIMARY KEY CHECK (id = 1),
			pos_x INTEGER NOT NULL,
			pos_y INTEGER NOT NULL
		)`,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range statements {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) savePosition(ctx context.Context, x, y int32) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO widget_state (id, pos_x, pos_y) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET pos_x = excluded.pos_x, pos_y = excluded.pos_y
	`, x, y)
	return err
}

func (a *App) windowManagerLoop(ctx context.Context) {
	time.Sleep(300 * time.Millisecond)
	_ = MakeWidgetWindow("winwid")

	titlePtr, _ := syscall.UTF16PtrFromString("winwid")
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))

	var lastX, lastY int32
	if hwnd != 0 {
		lastX, lastY, _ = GetWindowPosition(hwnd)
		if a.db != nil {
			var x, y int32
			err := a.db.QueryRowContext(ctx,
				`SELECT pos_x, pos_y FROM widget_state WHERE id = 1`).Scan(&x, &y)
			switch {
			case err == nil:
				SetWindowPosition(hwnd, x, y)
				lastX, lastY = x, y
			case !errors.Is(err, sql.ErrNoRows):
				log.Printf("Не удалось прочитать позицию: %v", err)
			}
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.closeDB()
			return
		case <-ticker.C:
			hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
			if hwnd == 0 {
				continue
			}
			SendToDesktop(hwnd)
			curX, curY, ok := GetWindowPosition(hwnd)
			if ok && (curX != lastX || curY != lastY) {
				lastX, lastY = curX, curY
				if a.db != nil {
					if err := a.savePosition(ctx, curX, curY); err != nil {
						log.Printf("Не удалось сохранить позицию: %v", err)
					}
				}
			}
		}
	}
}

// GetNotesByDate — заметки за выбранный день
func (a *App) GetNotesByDate(date string) ([]Note, error) {
	result := make([]Note, 0)
	if a.db == nil {
		return result, errNoDB
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	rows, err := a.db.QueryContext(ctx, `
		SELECT id, note_date, note_time, content, completed
		FROM notes WHERE note_date = ?
		ORDER BY note_time ASC, id ASC
	`, date)
	if err != nil {
		log.Printf("GetNotesByDate: %v", err)
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var n Note
		var completedInt int
		if err := rows.Scan(&n.ID, &n.Date, &n.Time, &n.Content, &completedInt); err != nil {
			log.Printf("GetNotesByDate scan: %v", err)
			return result, err
		}
		n.Completed = completedInt != 0
		result = append(result, n)
	}
	return result, rows.Err()
}

// AddNote валидация + вставка
func (a *App) AddNote(date, timeStr, content string) (Note, error) {
	content = strings.TrimSpace(content)
	if date == "" || timeStr == "" || content == "" {
		return Note{}, errors.New("дата, время и текст обязательны")
	}
	if a.db == nil {
		return Note{}, errNoDB
	}
	newNote := Note{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Date:    date,
		Time:    timeStr,
		Content: content,
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	completedInt := 0
	if newNote.Completed {
		completedInt = 1
	}
	if _, err := a.db.ExecContext(ctx, `
		INSERT INTO notes (id, note_date, note_time, content, completed)
		VALUES (?, ?, ?, ?, ?)
	`, newNote.ID, newNote.Date, newNote.Time, newNote.Content, completedInt); err != nil {
		log.Printf("AddNote: %v", err)
		return Note{}, err
	}
	return newNote, nil
}

// DeleteNote удаление по id
func (a *App) DeleteNote(id string) error {
	if a.db == nil {
		return errNoDB
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	if _, err := a.db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id); err != nil {
		log.Printf("DeleteNote: %v", err)
		return err
	}
	return nil
}
