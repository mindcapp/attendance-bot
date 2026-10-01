// Package excel генерирует XLSX-отчёт по явкам за день.
package excel

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/mindcapp/attendance-bot/storage"
)

// Build создаёт временный XLSX-файл с явками и возвращает путь к нему.
// Вызывающий код отвечает за удаление файла после отправки.
func Build(records []storage.Record) (string, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Явки"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return "", fmt.Errorf("создание листа: %w", err)
	}
	f.SetActiveSheet(idx)
	f.DeleteSheet("Sheet1")

	headers := []string{"Номер", "ФИ", "Статус", "Время отметки", "К какой паре"}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	if err != nil {
		return "", fmt.Errorf("создание стиля заголовка: %w", err)
	}
	cellStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	if err != nil {
		return "", fmt.Errorf("создание стиля ячеек: %w", err)
	}

	for i, hd := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, hd)
	}
	firstCell, _ := excelize.CoordinatesToCellName(1, 1)
	lastHeaderCell, _ := excelize.CoordinatesToCellName(len(headers), 1)
	if err := f.SetCellStyle(sheet, firstCell, lastHeaderCell, headerStyle); err != nil {
		return "", fmt.Errorf("применение стиля заголовка: %w", err)
	}

	for r, rec := range records {
		row := r + 2
		values := []interface{}{rec.Number, rec.Name, rec.Status, rec.Time, rec.Para}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			f.SetCellValue(sheet, cell, v)
		}
		startCell, _ := excelize.CoordinatesToCellName(1, row)
		endCell, _ := excelize.CoordinatesToCellName(len(headers), row)
		if err := f.SetCellStyle(sheet, startCell, endCell, cellStyle); err != nil {
			return "", fmt.Errorf("применение стиля строки %d: %w", row, err)
		}
	}

	widths := []float64{10, 26, 22, 14, 14}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(sheet, col, col, w); err != nil {
			return "", fmt.Errorf("установка ширины колонки %s: %w", col, err)
		}
	}

	path := filepath.Join(os.TempDir(), fmt.Sprintf("attendance_%d.xlsx", time.Now().UnixNano()))
	if err := f.SaveAs(path); err != nil {
		return "", fmt.Errorf("сохранение %s: %w", path, err)
	}
	return path, nil
}

func borderAll() []excelize.Border {
	sides := []string{"left", "top", "right", "bottom"}
	borders := make([]excelize.Border, len(sides))
	for i, s := range sides {
		borders[i] = excelize.Border{Type: s, Color: "000000", Style: 1}
	}
	return borders
}
