import './style.css';
import { GetNotesByDate, AddNote, DeleteNote } from '../wailsjs/go/main/App';

// форматирование даты в YYYY-MM-DD с учетом локального часового пояса
function formatISO(dateObj) {
    const y = dateObj.getFullYear();
    const m = String(dateObj.getMonth() + 1).padStart(2, '0');
    const d = String(dateObj.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
}

let currentToday = formatISO(new Date());
let selectedDate = currentToday;

document.querySelector('#app').innerHTML = `
  <div class="bento-grid">
    <!-- левая карточка: часы + календарь -->
    <div class="apple-card">
      <div class="drag-zone">
        <div class="time-display" id="clock">00:00</div>
        <div class="month-title" id="month-label">НОЯБРЬ</div>
      </div>
      <div class="calendar-weekdays">
        <span>П</span><span>В</span><span>С</span><span>Ч</span><span>П</span><span>С</span><span>В</span>
      </div>
      <div class="calendar-days" id="calendar-grid"></div>
    </div>

    <!-- правая карточка: расписание и заметки -->
    <div class="apple-card">
      <div class="schedule-header drag-zone">
        <span class="schedule-title" id="selected-date-title">Сегодня</span>
        <span class="schedule-count" id="notes-count">0</span>
      </div>
      <div class="notes-list" id="notes"></div>
      <div class="input-capsule">
        <input type="time" id="note-time" class="time-in" value="10:00" />
        <input type="text" id="note-text" class="text-in" placeholder="Новое событие..." />
        <button class="add-btn" id="add-btn">+</button>
      </div>
    </div>
  </div>
`;

function updateClock() {
    const now = new Date();
    document.getElementById('clock').textContent = now.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit'
    });
    document.getElementById('month-label').textContent = now.toLocaleDateString('ru-RU', {
        weekday: 'short',
        day: 'numeric',
        month: 'short'
    }).replace('.', '');

    // проверяем не наступила ли полночь
    const newToday = formatISO(now);
    if (newToday !== currentToday) {
        // если у пользователя был открыт раздел сегодня переключаем его на новый день
        if (selectedDate === currentToday) {
            selectedDate = newToday;
            document.getElementById('selected-date-title').textContent = 'Сегодня';
            renderNotes();
        }
        currentToday = newToday;
        renderCalendar(); // перерисовываем красный кружок на новое число
    }
}
setInterval(updateClock, 1000);
updateClock();

function renderCalendar() {
    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth();
    const todayStr = formatISO(now);

    const firstDay = new Date(year, month, 1).getDay();
    const offset = (firstDay + 6) % 7; // сдвиг чтобы неделя начиналась с понедельника
    const daysInMonth = new Date(year, month + 1, 0).getDate();

    const grid = document.getElementById('calendar-grid');
    let html = '';

    for (let i = 0; i < offset; i++) {
        html += `<div class="day-cell empty"></div>`;
    }

    for (let day = 1; day <= daysInMonth; day++) {
        const dateStr = formatISO(new Date(year, month, day));
        const isToday = dateStr === todayStr ? 'today' : '';
        const isSelected = dateStr === selectedDate ? 'selected' : '';
        html += `<div class="day-cell ${isToday} ${isSelected}" data-date="${dateStr}">${day}</div>`;
    }

    grid.innerHTML = html;

    grid.querySelectorAll('.day-cell[data-date]').forEach(cell => {
        cell.addEventListener('click', () => {
            selectedDate = cell.dataset.date;
            const isToday = selectedDate === todayStr;
            document.getElementById('selected-date-title').textContent = isToday
                ? 'Сегодня'
                : new Date(selectedDate).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
            renderCalendar();
            renderNotes();
        });
    });
}

function showError(message) {
    document.getElementById('notes').innerHTML = `<div class="empty-state">${message}</div>`;
}

async function renderNotes() {
    let notes;
    try {
        notes = (await GetNotesByDate(selectedDate)) || [];
    } catch (err) {
        console.error(err);
        showError('Нет доступа к базе данных');
        return;
    }
    // сортируем события по времени например 09:00 раньше 14:30
    notes.sort((a, b) => a.time.localeCompare(b.time));

    document.getElementById('notes-count').textContent = notes.length;
    const container = document.getElementById('notes');

    if (notes.length === 0) {
        container.innerHTML = `<div class="empty-state">Нет событий</div>`;
        return;
    }

    container.innerHTML = notes.map(n => `
        <div class="note-item">
            <div class="check-circle" data-id="${n.id}" title="Выполнено"></div>
            <div class="note-info">
                <span class="note-text">${n.content}</span>
                <span class="note-time">${n.time}</span>
            </div>
        </div>
    `).join('');

    container.querySelectorAll('.check-circle').forEach(btn => {
        btn.addEventListener('click', async () => {
            try {
                await DeleteNote(btn.dataset.id);
            } catch (err) {
                console.error(err);
                showError('Не удалось удалить событие');
                setTimeout(renderNotes, 2500);
                return;
            }
            await renderNotes();
        });
    });
}

async function handleAdd() {
    const timeEl = document.getElementById('note-time');
    const textEl = document.getElementById('note-text');
    if (!textEl.value.trim()) return;

    try {
        await AddNote(selectedDate, timeEl.value, textEl.value.trim());
    } catch (err) {
        // текст остается в поле, чтобы его можно было отправить повторно
        console.error(err);
        showError('Не удалось сохранить событие');
        setTimeout(renderNotes, 2500);
        return;
    }
    textEl.value = '';
    await renderNotes();
}

document.getElementById('add-btn').addEventListener('click', handleAdd);
document.getElementById('note-text').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') handleAdd();
});

renderCalendar();
renderNotes();