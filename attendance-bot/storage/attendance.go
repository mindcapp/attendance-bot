package storage

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/yourusername/attendance-bot/metrics"
)

const dateLayout = "2006-01-02"

var attendanceHeader = []string{"date", "time", "number", "name", "status", "para", "delay_minutes"}

// Record описывает одну отметку явки за день.
type Record struct {
	Date         string
	Time         string
	Number       int
	Name         string
	Status       string // "Пришёл", "Опоздаю", "Не приду", "По уважительной", "Задержался на N минут"
	Para         string // номер пары ("-", если не применимо)
	DelayMinutes int
}

type Attendance struct {
	mu   sync.Mutex
	path string
}

func NewAttendance(path string) *Attendance {
	return &Attendance{path: path}
}

// Save записывает явку. Если на эту дату у этого номера уже есть запись,
// она заменяется новой — студент может изменить статус в течение дня.
func (a *Attendance) Save(rec Record) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	records, err := a.readAll()
	if err != nil {
		return err
	}

	out := records[:0]
	for _, r := range records {
		if r.Date == rec.Date && r.Number == rec.Number {
			continue
		}
		out = append(out, r)
	}
	out = append(out, rec)

	if err := a.writeAll(out); err != nil {
		return err
	}
	metrics.TotalCheckins.Add(1)
	return nil
}

// Today возвращает сегодняшние записи, отсортированные по номеру.
func (a *Attendance) Today(now time.Time) ([]Record, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	records, err := a.readAll()
	if err != nil {
		return nil, err
	}

	date := now.Format(dateLayout)
	var out []Record
	for _, r := range records {
		if r.Date == date {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func (a *Attendance) readAll() ([]Record, error) {
	f, err := os.Open(a.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("открытие %s: %w", a.path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("чтение %s: %w", a.path, err)
	}

	var out []Record
	for i, row := range rows {
		if i == 0 && len(row) > 0 && row[0] == "date" {
			continue // заголовок
		}
		if len(row) < 7 {
			continue
		}
		number, _ := strconv.Atoi(row[2])
		delay, _ := strconv.Atoi(row[6])
		out = append(out, Record{
			Date:         row[0],
			Time:         row[1],
			Number:       number,
			Name:         row[3],
			Status:       row[4],
			Para:         row[5],
			DelayMinutes: delay,
		})
	}
	return out, nil
}

func (a *Attendance) writeAll(records []Record) error {
	sort.Slice(records, func(i, j int) bool {
		if records[i].Date != records[j].Date {
			return records[i].Date < records[j].Date
		}
		return records[i].Number < records[j].Number
	})

	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("открытие %s: %w", a.path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(attendanceHeader); err != nil {
		return err
	}
	for _, r := range records {
		row := []string{
			r.Date, r.Time, strconv.Itoa(r.Number), r.Name,
			r.Status, r.Para, strconv.Itoa(r.DelayMinutes),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("запись %s: %w", a.path, err)
	}
	return nil
}
