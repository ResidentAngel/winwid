package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Note структура одной заметки или задачи в расписании
// теги json нужны для сериализации при передаче во фронт и записи в файл
type Note struct {
	ID        string `json:"id"`
	Date      string `json:"date"` // формат yyyy-mm-dd для привязки к календарю
	Time      string `json:"time"` // например 14:30
	Content   string `json:"content"`
	Completed bool   `json:"completed"`
}

type App struct {
	ctx      context.Context
	mu       sync.RWMutex // защищает срез notes от состояния гонки
	notes    []Note
	dataFile string
}

func NewApp() *App {
	return &App{
		notes:    make([]Note, 0),
		dataFile: "data/notes.json",
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = a.loadFromDisk()

	go func() {
		// ждем создания окна и превращаем его в виджет
		time.Sleep(300 * time.Millisecond)
		_ = MakeWidgetWindow("winwid")

		// каждые 2 секунды закрепляем виджет на нижнем слое рабочего стола,
		// чтобы после клика по нему он снова ложился под открытые программы
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		titlePtr, _ := syscall.UTF16PtrFromString("winwid")
		for {
			select {
			case <-ctx.Done():
				return // Graceful shutdown при закрытии программы
			case <-ticker.C:
				hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
				if hwnd != 0 {
					SendToDesktop(hwnd)
				}
			}
		}
	}()
}

// DeleteNote удаляет выполненную заметку по её ID
func (a *App) DeleteNote(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, n := range a.notes {
		if n.ID == id {
			// идиоматичное удаление элемента из среза в Go
			a.notes = append(a.notes[:i], a.notes[i+1:]...)
			break
		}
	}
	_ = a.saveToDisk()
}

// GetNotesByDate возвращает заметки на конкретный день
func (a *App) GetNotesByDate(date string) []Note {
	a.mu.RLock()         // блокируем только на чтение
	defer a.mu.RUnlock() // гарантированно снимаем блокировку при выходе из функции
	var result []Note
	for _, n := range a.notes {
		if n.Date == date {
			result = append(result, n)
		}
	}
	return result
}

// AddNote добавляет новую заметку в календарь
func (a *App) AddNote(date, timeStr string, content string) Note {
	a.mu.Lock()
	defer a.mu.Unlock()

	newNote := Note{
		ID:      time.Now().Format("20060102150405.000"), // простой уникальный ID ПО таймстампу
		Date:    date,
		Time:    timeStr,
		Content: content,
	}

	a.notes = append(a.notes, newNote)
	_ = a.saveToDisk()
	return newNote
}

func (a *App) loadFromDisk() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := os.ReadFile(a.dataFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &a.notes)
}

func (a *App) saveToDisk() error {
	data, err := json.MarshalIndent(a.notes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.dataFile, data, 0644)
}
