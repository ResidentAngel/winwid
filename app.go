package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Note struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Time      string `json:"time"`
	Content   string `json:"content"`
	Completed bool   `json:"completed"`
}

type App struct {
	ctx context.Context
	db  *pgxpool.Pool // безопасный пул соединений с PostgreSQL
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// ,ерем строку подключения из переменной окружения или используем дефолтную
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:secret@127.0.0.1:5432/winwid?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Printf("Ошибка подключения к PostgreSQL: %v", err)
	} else {
		a.db = pool
		a.initSchema()
	}

	go a.windowManagerLoop(ctx)
}

// initSchema автоматически накатывает таблицы при старте
func (a *App) initSchema() {
	query := `
	CREATE TABLE IF NOT EXISTS notes (
		id TEXT PRIMARY KEY,
		note_date TEXT NOT NULL,
		note_time TEXT NOT NULL,
		content TEXT NOT NULL,
		completed BOOLEAN DEFAULT FALSE
	);
	CREATE INDEX IF NOT EXISTS idx_notes_date ON notes(note_date);

	CREATE TABLE IF NOT EXISTS widget_state (
		id INT PRIMARY KEY DEFAULT 1,
		pos_x INT NOT NULL,
		pos_y INT NOT NULL
	);`

	if _, err := a.db.Exec(a.ctx, query); err != nil {
		log.Printf("Ошибка миграции БД: %v", err)
	}
}

// windowManagerLoop закрепляет виджет на рабочем столе и сохраняет его позицию при перетаскивании
func (a *App) windowManagerLoop(ctx context.Context) {
	time.Sleep(300 * time.Millisecond)
	_ = MakeWidgetWindow("winwid")

	titlePtr, _ := syscall.UTF16PtrFromString("winwid")
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))

	// восстанавливаем сохраненную позицию из PostgreSQL при старте
	var lastX, lastY int32
	if a.db != nil && hwnd != 0 {
		err := a.db.QueryRow(ctx, `SELECT pos_x, pos_y FROM widget_state WHERE id = 1`).Scan(&lastX, &lastY)
		if err == nil {
			SetWindowPosition(hwnd, lastX, lastY)
		} else {
			lastX, lastY, _ = GetWindowPosition(hwnd)
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if a.db != nil {
				a.db.Close() // корректно закрываем пул соединений при выходе
			}
			return
		case <-ticker.C:
			hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
			if hwnd == 0 {
				continue
			}

			SendToDesktop(hwnd)

			// проверяем перетащил ли пользователь виджет в другое место
			curX, curY, ok := GetWindowPosition(hwnd)
			if ok && (curX != lastX || curY != lastY) {
				lastX, lastY = curX, curY
				if a.db != nil {
					_, _ = a.db.Exec(ctx, `
						INSERT INTO widget_state (id, pos_x, pos_y)
						VALUES (1, $1, $2)
						ON CONFLICT (id) DO UPDATE SET pos_x = EXCLUDED.pos_x, pos_y = EXCLUDED.pos_y
					`, curX, curY)
				}
			}
		}
	}
}

// GetNotesByDate читает заметки за выбранный день из PostgreSQL
func (a *App) GetNotesByDate(date string) []Note {
	result := make([]Note, 0)
	if a.db == nil {
		return result
	}

	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
	defer cancel()

	rows, err := a.db.Query(ctx, `
		SELECT id, note_date, note_time, content, completed
		FROM notes
		WHERE note_date = $1
		ORDER BY note_time ASC
	`, date)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Date, &n.Time, &n.Content, &n.Completed); err == nil {
			result = append(result, n)
		}
	}
	return result
}

// AddNote сохраняет новую заметку в PostgreSQL
func (a *App) AddNote(date, timeStr, content string) Note {
	newNote := Note{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Date:    date,
		Time:    timeStr,
		Content: content,
	}

	if a.db != nil {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
		defer cancel()

		_, _ = a.db.Exec(ctx, `
			INSERT INTO notes (id, note_date, note_time, content, completed)
			VALUES ($1, $2, $3, $4, $5)
		`, newNote.ID, newNote.Date, newNote.Time, newNote.Content, newNote.Completed)
	}
	return newNote
}

// DeleteNote удаляет заметку из PostgreSQL
func (a *App) DeleteNote(id string) {
	if a.db == nil {
		return
	}

	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
	defer cancel()

	_, _ = a.db.Exec(ctx, `DELETE FROM notes WHERE id = $1`, id)
}
