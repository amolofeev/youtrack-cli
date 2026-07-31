package output

import (
	"errors"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
)

const (
	tablePadding = 2
	minColWidth  = 4
)

// Table печатает таблицу на text/tabwriter (SPEC §4.3). Пустые rows не выводят
// ничего — сообщения об отсутствии данных печатают сами команды. Многострочные
// и длинные поля переносятся и обрезаются по ширине терминала (если она известна).
// В режиме ModeJSON возвращает ошибку: для --json используется JSON.
func (p *Printer) Table(headers []string, rows [][]string) error {
	if p.mode == ModeJSON {
		return errors.New("table output is not available in JSON mode")
	}
	if len(rows) == 0 {
		return nil
	}
	widths := colWidths(headers, rows, p.width)
	w := tabwriter.NewWriter(p.out, 0, 0, tablePadding, ' ', 0)

	writeRow := func(cells []string) error {
		var err error
		for j, cell := range cells {
			if j > 0 {
				_, err = w.Write([]byte{'\t'})
				if err != nil {
					return err
				}
			}
			if _, err = w.Write([]byte(cell)); err != nil {
				return err
			}
		}
		_, err = w.Write([]byte{'\n'})
		return err
	}

	if err := writeExpanded(writeRow, headers, widths); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeExpanded(writeRow, row, widths); err != nil {
			return err
		}
	}
	return w.Flush()
}

// writeExpanded разворачивает многострочные ячейки в отдельные физические строки
// таблицы, чтобы переводы строк внутри ячейки не ломали колонки.
func writeExpanded(writeRow func([]string) error, cells []string, widths []int) error {
	wrapped := make([][]string, len(cells))
	height := 1
	for j, cell := range cells {
		lines := wrapText(cell, colWidth(widths, j))
		wrapped[j] = lines
		if len(lines) > height {
			height = len(lines)
		}
	}
	for i := 0; i < height; i++ {
		line := make([]string, len(cells))
		for j := range cells {
			if i < len(wrapped[j]) {
				line[j] = wrapped[j][i]
			}
		}
		if err := writeRow(line); err != nil {
			return err
		}
	}
	return nil
}

func colWidth(widths []int, j int) int {
	if j < len(widths) {
		return widths[j]
	}
	return 0
}

// colWidths вычисляет ширины колонок по содержимому; если известна ширина
// терминала и таблица не влезает — колонки сжимаются пропорционально.
func colWidths(headers []string, rows [][]string, termWidth int) []int {
	n := len(headers)
	if n == 0 && len(rows) > 0 {
		n = len(rows[0])
	}
	if n == 0 {
		return nil
	}
	widths := make([]int, n)
	update := func(cells []string) {
		for j, cell := range cells {
			if j >= n {
				continue
			}
			if w := lineWidth(cell); w > widths[j] {
				widths[j] = w
			}
		}
	}
	update(headers)
	for _, row := range rows {
		update(row)
	}
	if termWidth > 0 {
		avail := termWidth - (n-1)*tablePadding
		total := 0
		for _, w := range widths {
			total += w
		}
		if total > avail {
			scale := float64(avail) / float64(total)
			for j := range widths {
				w := int(float64(widths[j]) * scale)
				if w < minColWidth {
					w = minColWidth
				}
				widths[j] = w
			}
		}
	}
	return widths
}

// lineWidth возвращает ширину самой длинной физической строки ячейки.
func lineWidth(cell string) int {
	max := 0
	for _, line := range wrapText(cell, 0) {
		if w := displayWidth(line); w > max {
			max = w
		}
	}
	return max
}

// wrapText разбивает ячейку на строки заданной ширины, перенося слова и жёстко
// обрезая слишком длинные слова. width <= 0 означает «без ограничения» (тогда
// ячейка разбивается только по переводам строк).
func wrapText(s string, width int) []string {
	if !strings.ContainsRune(s, '\n') && (width <= 0 || displayWidth(s) <= width) {
		return []string{s}
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		if width <= 0 {
			lines = append(lines, para)
			continue
		}
		lines = append(lines, wrapLine(para, width)...)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// wrapLine переносит одну строку без переводов на ширину width.
func wrapLine(line string, width int) []string {
	if displayWidth(line) <= width {
		return []string{line}
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	cur := ""
	for _, w := range words {
		for displayWidth(w) > width {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			out = append(out, truncateRunes(w, width))
			w = string([]rune(w)[width:])
		}
		switch {
		case cur == "":
			cur = w
		case displayWidth(cur)+1+displayWidth(w) <= width:
			cur += " " + w
		default:
			out = append(out, cur)
			cur = w
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
