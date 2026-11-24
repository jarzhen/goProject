package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/lxn/walk"
)

const (
	modeCalendar = "calendar"
	modeHours    = "hours"
	dateLayout   = "2006-01-02"
)

type appStyle struct {
	fonts struct {
		title       *walk.Font
		head        *walk.Font
		hint        *walk.Font
		pill        *walk.Font
		dayActive   *walk.Font
		dayIdle     *walk.Font
		entryActive *walk.Font
		entryIdle   *walk.Font
		totalActive *walk.Font
		totalIdle   *walk.Font
	}

	colors  map[string]walk.Color
	brushes map[string]walk.Brush
}

func newAppStyle() (*appStyle, error) {
	s := &appStyle{
		colors: map[string]walk.Color{
			"bg":            walk.RGB(0xee, 0xf1, 0xf7),
			"panel":         walk.RGB(0xff, 0xff, 0xff),
			"headerBg":      walk.RGB(0x4a, 0x6f, 0xa5),
			"headerBgWe":    walk.RGB(0x6b, 0x87, 0xb5),
			"headerFg":      walk.RGB(0xff, 0xff, 0xff),
			"fg":            walk.RGB(0x33, 0x33, 0x33),
			"weekendFg":     walk.RGB(0xc0, 0x39, 0x2b),
			"todayBg":       walk.RGB(0xff, 0xe0, 0x8a),
			"totalBg":       walk.RGB(0xff, 0xf6, 0xd9),
			"totalFg":       walk.RGB(0x8a, 0x6d, 0x00),
			"btnBg":         walk.RGB(0x4a, 0x6f, 0xa5),
			"btnActive":     walk.RGB(0x3c, 0x5c, 0x8a),
			"btn2Bg":        walk.RGB(0x5c, 0xb8, 0x5c),
			"btn2Active":    walk.RGB(0x4a, 0x9e, 0x4a),
			"invalidBg":     walk.RGB(0xff, 0xe3, 0xe3),
			"invalidFg":     walk.RGB(0xc0, 0x39, 0x2b),
			"dayActiveBg":   walk.RGB(0xdb, 0xe9, 0xfb),
			"dayActiveFg":   walk.RGB(0x1f, 0x4e, 0x79),
			"entryActiveBg": walk.RGB(0xff, 0xff, 0xff),
			"entryActiveFg": walk.RGB(0x1f, 0x4e, 0x79),
			"entryIdleBg":   walk.RGB(0xf3, 0xf5, 0xf9),
			"entryIdleFg":   walk.RGB(0xa7, 0xae, 0xba),
			"dayIdleFg":     walk.RGB(0xb9, 0xc0, 0xcc),
			"focusBg":       walk.RGB(0xea, 0xf2, 0xff),
			"hintFg":        walk.RGB(0x7b, 0x84, 0x94),
		},
		brushes: make(map[string]walk.Brush),
	}

	fonts := []struct {
		field **walk.Font
		size  int
		style walk.FontStyle
	}{
		{&s.fonts.title, 11, walk.FontBold},
		{&s.fonts.head, 11, walk.FontBold},
		{&s.fonts.hint, 9, 0},
		{&s.fonts.pill, 9, walk.FontBold},
		{&s.fonts.dayActive, 17, walk.FontBold},
		{&s.fonts.dayIdle, 8, 0},
		{&s.fonts.entryActive, 15, walk.FontBold},
		{&s.fonts.entryIdle, 9, 0},
		{&s.fonts.totalActive, 15, walk.FontBold},
		{&s.fonts.totalIdle, 10, walk.FontBold},
	}
	for _, f := range fonts {
		font, err := walk.NewFont("Microsoft YaHei UI", f.size, f.style)
		if err != nil {
			return nil, err
		}
		*f.field = font
	}
	for name, color := range s.colors {
		brush, err := walk.NewSolidColorBrush(color)
		if err != nil {
			return nil, err
		}
		s.brushes[name] = brush
	}
	return s, nil
}

func (s *appStyle) color(name string) walk.Color { return s.colors[name] }
func (s *appStyle) brush(name string) walk.Brush { return s.brushes[name] }

type dayCell struct {
	label   *walk.Label
	weekend bool
	today   bool
}

type weekRow struct {
	days    []dayCell
	entries []*walk.LineEdit
	total   *walk.Label
}

type calendarApp struct {
	win *walk.Dialog

	yearEdit  *walk.LineEdit
	monthEdit *walk.LineEdit
	generate  *walk.PushButton
	toggle    *walk.PushButton
	copyBtn   *walk.PushButton
	pill      *walk.Label
	hint      *walk.Label
	groupBox    *walk.GroupBox
	groupLayout *walk.BoxLayout
	grid        *walk.Composite
	gridLayout  *walk.GridLayout

	rows       []weekRow
	grandLabel *walk.Label
	mode       string
	hourStore  map[string]string

	provider    *holidayProvider
	holidayData holidayData
	year        int
	month       int

	style *appStyle
}

func newCalendarApp() (*calendarApp, error) {
	mw, err := walk.NewDialog(nil)
	if err != nil {
		return nil, err
	}
	style, err := newAppStyle()
	if err != nil {
		return nil, err
	}

	a := &calendarApp{
		win:        mw,
		mode:       modeCalendar,
		hourStore:  make(map[string]string),
		provider:   newHolidayProvider(),
		style:      style,
	}

	mw.SetTitle("日历 + 工时填写")
	_ = mw.SetSize(walk.Size{Width: 1180, Height: 660})
	mw.SetBackground(style.brush("bg"))

	mainLayout := walk.NewVBoxLayout()
	mainLayout.SetMargins(walk.Margins{HNear: 12, VNear: 12, HFar: 12, VFar: 12})
	mainLayout.SetSpacing(6)
	_ = mw.SetLayout(mainLayout)

	if err := a.buildToolbar(); err != nil {
		return nil, err
	}
	if err := a.buildBody(mainLayout); err != nil {
		return nil, err
	}

	today := time.Now()
	_ = a.yearEdit.SetText(strconv.Itoa(today.Year()))
	_ = a.monthEdit.SetText(strconv.Itoa(int(today.Month())))
	a.rebuild()
	return a, nil
}

func (a *calendarApp) buildToolbar() error {
	bar, err := walk.NewComposite(a.win)
	if err != nil {
		return err
	}
	bar.SetBackground(a.style.brush("bg"))
	barLayout := walk.NewHBoxLayout()
	barLayout.SetMargins(walk.Margins{HNear: 0, VNear: 0, HFar: 0, VFar: 0})
	barLayout.SetSpacing(6)
	_ = bar.SetLayout(barLayout)

	box, err := walk.NewComposite(bar)
	if err != nil {
		return err
	}
	box.SetBackground(a.style.brush("panel"))
	boxLayout := walk.NewHBoxLayout()
	boxLayout.SetMargins(walk.Margins{HNear: 6, VNear: 4, HFar: 6, VFar: 4})
	boxLayout.SetSpacing(4)
	_ = box.SetLayout(boxLayout)

	if _, err := a.addLabel(box, "年", a.style.fonts.title, a.style.color("fg"), ""); err != nil {
		return err
	}
	if a.yearEdit, err = a.addLineEdit(box, 120); err != nil {
		return err
	}
	if _, err := a.addLabel(box, "月", a.style.fonts.title, a.style.color("fg"), ""); err != nil {
		return err
	}
	if a.monthEdit, err = a.addLineEdit(box, 72); err != nil {
		return err
	}
	for _, edit := range []*walk.LineEdit{a.yearEdit, a.monthEdit} {
		edit.KeyDown().Attach(func(key walk.Key) {
			if key == walk.KeyReturn {
				a.rebuild()
			}
		})
	}

	if a.generate, err = a.addButton(bar, "生成日历", a.style.brush("btnBg"), a.rebuild); err != nil {
		return err
	}
	if a.toggle, err = a.addButton(bar, "", a.style.brush("btnBg"), a.toggleMode); err != nil {
		return err
	}
	if a.copyBtn, err = a.addButton(bar, "复制工时累计", a.style.brush("btn2Bg"), a.copyTotals); err != nil {
		return err
	}

	a.pill, err = walk.NewLabel(bar)
	if err != nil {
		return err
	}
	a.pill.SetTextAlignment(walk.AlignCenter)
	a.pill.SetFont(a.style.fonts.pill)
	a.pill.SetBackground(a.style.brush("dayActiveBg"))
	a.pill.SetTextColor(a.style.color("dayActiveFg"))
	return nil
}

func (a *calendarApp) buildBody(mainLayout *walk.BoxLayout) error {
	var err error
	a.hint, err = walk.NewLabel(a.win)
	if err != nil {
		return err
	}
	a.hint.SetText("表格上半部分为日期（只读），下半部分为工时（可编辑）；右侧“工时”列为该行累计")
	a.hint.SetFont(a.style.fonts.hint)
	a.hint.SetTextColor(a.style.color("hintFg"))

	a.groupBox, err = walk.NewGroupBox(a.win)
	if err != nil {
		return err
	}
	a.groupBox.SetFont(a.style.fonts.title)
	a.groupBox.SetBackground(a.style.brush("panel"))
	a.groupLayout = walk.NewVBoxLayout()
	a.groupLayout.SetMargins(walk.Margins{HNear: 8, VNear: 8, HFar: 8, VFar: 8})
	a.groupLayout.SetSpacing(0)
	_ = a.groupBox.SetLayout(a.groupLayout)
	_ = mainLayout.SetStretchFactor(a.groupBox, 1)

	return a.createGrid()
}

func (a *calendarApp) createGrid() error {
	var err error
	a.grid, err = walk.NewComposite(a.groupBox)
	if err != nil {
		return err
	}
	a.grid.SetBackground(a.style.brush("panel"))
	a.gridLayout = walk.NewGridLayout()
	a.gridLayout.SetMargins(walk.Margins{HNear: 2, VNear: 2, HFar: 2, VFar: 2})
	a.gridLayout.SetSpacing(1)
	_ = a.grid.SetLayout(a.gridLayout)
	for c := 0; c < 8; c++ {
		_ = a.gridLayout.SetColumnStretchFactor(c, 1)
	}
	_ = a.groupLayout.SetStretchFactor(a.grid, 1)
	return nil
}

func (a *calendarApp) addLabel(parent walk.Container, text string, font *walk.Font, fg walk.Color, bgName string) (*walk.Label, error) {
	label, err := walk.NewLabel(parent)
	if err != nil {
		return nil, err
	}
	_ = label.SetText(text)
	label.SetFont(font)
	label.SetTextColor(fg)
	if bgName != "" {
		label.SetBackground(a.style.brush(bgName))
	}
	label.SetTextAlignment(walk.AlignCenter)
	return label, nil
}

func (a *calendarApp) addLineEdit(parent walk.Container, width int) (*walk.LineEdit, error) {
	edit, err := walk.NewLineEdit(parent)
	if err != nil {
		return nil, err
	}
	edit.SetFont(a.style.fonts.title)
	_ = edit.SetTextAlignment(walk.AlignCenter)
	if width > 0 {
		_ = edit.SetWidth(width)
		_ = edit.SetMinMaxSize(walk.Size{Width: width, Height: 0}, walk.Size{})
	}
	return edit, nil
}

func (a *calendarApp) addButton(parent walk.Container, text string, bg walk.Brush, onClick func()) (*walk.PushButton, error) {
	btn, err := walk.NewPushButton(parent)
	if err != nil {
		return nil, err
	}
	_ = btn.SetText(text)
	btn.SetFont(a.style.fonts.title)
	btn.SetBackground(bg)
	btn.Clicked().Attach(func() { onClick() })
	return btn, nil
}

func (a *calendarApp) addToGrid(widget walk.Widget, layout *walk.GridLayout, x, y, width, height int) error {
	return layout.SetRange(widget, walk.Rectangle{X: x, Y: y, Width: width, Height: height})
}

func (a *calendarApp) rebuild() {
	year, err1 := strconv.Atoi(strings.TrimSpace(a.yearEdit.Text()))
	month, err2 := strconv.Atoi(strings.TrimSpace(a.monthEdit.Text()))
	if err1 != nil || err2 != nil || month < 1 || month > 12 {
		walk.MsgBox(a.win, "错误", "请输入有效的年份和月份（月份 1-12）", walk.MsgBoxOK|walk.MsgBoxIconError)
		return
	}
	a.year, a.month = year, month
	_ = a.groupBox.SetTitle(fmt.Sprintf("日历 + 工时（%d年%d月）", year, month))

	data := a.provider.get(year)
	a.holidayData = *data
	a.hint.SetText(fmt.Sprintf("表格上半部分为日期（只读），下半部分为工时（可编辑）；右侧“工时”列为该行累计 ｜ %d年节假日数据来源：%s", year, a.provider.source))

	if a.grid != nil {
		_ = a.groupBox.Children().Remove(a.grid)
		a.grid.Dispose()
	}
	a.rows = nil
	a.grandLabel = nil
	if err := a.createGrid(); err != nil {
		return
	}
	layout := a.gridLayout

	weeks := monthCalendar(year, month)
	today := time.Now()
	weekHeaders := []string{"一", "二", "三", "四", "五", "六", "日"}
	for c, name := range weekHeaders {
		bg := "headerBg"
		if c >= 5 {
			bg = "headerBgWe"
		}
		label, err := a.addLabel(a.grid, name, a.style.fonts.head, a.style.color("headerFg"), bg)
		if err != nil {
			return
		}
		_ = a.addToGrid(label, layout, c, 0, 1, 1)
	}
	totalHeader, err := a.addLabel(a.grid, "工时", a.style.fonts.head, a.style.color("totalFg"), "totalBg")
	if err != nil {
		return
	}
	_ = a.addToGrid(totalHeader, layout, 7, 0, 1, 1)

	for r, week := range weeks {
		a.buildWeekRow(layout, r, week, year, month, today)
		_ = layout.SetRowStretchFactor(r+1, 1)
	}

	grandTitle, err := a.addLabel(a.grid, "合计", a.style.fonts.head, a.style.color("totalFg"), "totalBg")
	if err != nil {
		return
	}
	_ = a.addToGrid(grandTitle, layout, 7, len(weeks)+1, 1, 1)
	a.grandLabel, err = a.addLabel(a.grid, "0", a.style.fonts.totalActive, a.style.color("totalFg"), "totalBg")
	if err != nil {
		return
	}
	_ = a.addToGrid(a.grandLabel, layout, 0, len(weeks)+1, 7, 1)

	a.updateAllTotals()
	a.applyEmphasis()
}

func (a *calendarApp) buildWeekRow(layout *walk.GridLayout, rowIndex int, week []int, year, month int, today time.Time) {
	row := weekRow{}
	for c, day := range week {
		cell, err := walk.NewComposite(a.grid)
		if err != nil {
			continue
		}
		cell.SetBackground(a.style.brush("panel"))
		cellLayout := walk.NewVBoxLayout()
		cellLayout.SetMargins(walk.Margins{HNear: 1, VNear: 1, HFar: 1, VFar: 1})
		cellLayout.SetSpacing(0)
		_ = cell.SetLayout(cellLayout)
		_ = a.addToGrid(cell, layout, c, rowIndex+1, 1, 1)

		if day == 0 {
			blank, _ := a.addLabel(cell, "", a.style.fonts.dayActive, a.style.color("panel"), "panel")
			blank2, _ := a.addLabel(cell, "", a.style.fonts.entryActive, a.style.color("panel"), "panel")
			_ = blank
			_ = blank2
			continue
		}

		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
		dateKey := date.Format(dateLayout)
		isToday := year == today.Year() && month == int(today.Month()) && day == today.Day()
		weekend := c >= 5

		dayLabel, err := a.addLabel(cell, strconv.Itoa(day), a.style.fonts.dayActive,
			a.style.color("dayActiveFg"), "panel")
		if err != nil {
			continue
		}
		if weekend {
			dayLabel.SetTextColor(a.style.color("weekendFg"))
		}
		if isToday {
			dayLabel.SetBackground(a.style.brush("todayBg"))
		}

		defaultText := ""
		if isWorkday(date, &a.holidayData) {
			defaultText = "8"
		}
		if saved, ok := a.hourStore[dateKey]; ok {
			defaultText = saved
		}
		entry, err := a.addLineEdit(cell, 0)
		if err != nil {
			continue
		}
		entry.SetFont(a.style.fonts.entryActive)
		_ = entry.SetText(defaultText)
		a.restoreEntryStyle(entry)

		rowIndexCopy := rowIndex
		entry.TextChanged().Attach(func() {
			a.hourStore[dateKey] = entry.Text()
			a.restoreEntryStyle(entry)
			a.updateRowTotal(rowIndexCopy)
		})
		entry.FocusedChanged().Attach(func() {
			if entry.Focused() {
				entry.SetBackground(a.style.brush("focusBg"))
			} else {
				a.restoreEntryStyle(entry)
			}
		})

		row.days = append(row.days, dayCell{label: dayLabel, weekend: weekend, today: isToday})
		row.entries = append(row.entries, entry)
	}

	total, err := a.addLabel(a.grid, "0", a.style.fonts.totalActive, a.style.color("totalFg"), "totalBg")
	if err != nil {
		return
	}
	_ = a.addToGrid(total, layout, 7, rowIndex+1, 1, 1)
	row.total = total
	a.rows = append(a.rows, row)
}

func monthCalendar(year, month int) [][]int {
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	offset := (int(first.Weekday()) + 6) % 7
	days := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.Local).Day()

	weeks := make([][]int, 0, 6)
	week := make([]int, 7)
	pos := offset
	for day := 1; day <= days; day++ {
		week[pos] = day
		pos++
		if pos == 7 {
			weeks = append(weeks, week)
			week = make([]int, 7)
			pos = 0
		}
	}
	if pos != 0 {
		weeks = append(weeks, week)
	}
	return weeks
}

func parseHours(text string) (float64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, true
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func fmtNum(value float64) string {
	if value == math.Trunc(value) {
		return strconv.Itoa(int(value))
	}
	return strconv.FormatFloat(math.Round(value*100)/100, 'f', -1, 64)
}

func (a *calendarApp) restoreEntryStyle(entry *walk.LineEdit) {
	if _, ok := parseHours(entry.Text()); !ok {
		entry.SetBackground(a.style.brush("invalidBg"))
		entry.SetTextColor(a.style.color("invalidFg"))
	} else if a.mode == modeHours {
		entry.SetBackground(a.style.brush("entryActiveBg"))
		entry.SetTextColor(a.style.color("entryActiveFg"))
	} else {
		entry.SetBackground(a.style.brush("entryIdleBg"))
		entry.SetTextColor(a.style.color("entryIdleFg"))
	}
	if a.mode == modeHours {
		entry.SetFont(a.style.fonts.entryActive)
	} else {
		entry.SetFont(a.style.fonts.entryIdle)
	}
}

func (a *calendarApp) updateRowTotal(row int) {
	if row < 0 || row >= len(a.rows) {
		return
	}
	total := 0.0
	for _, entry := range a.rows[row].entries {
		if value, ok := parseHours(entry.Text()); ok {
			total += value
		}
	}
	_ = a.rows[row].total.SetText(fmtNum(total))
	a.updateGrandTotal()
}

func (a *calendarApp) updateGrandTotal() {
	if a.grandLabel == nil {
		return
	}
	grand := 0.0
	for _, row := range a.rows {
		for _, entry := range row.entries {
			if value, ok := parseHours(entry.Text()); ok {
				grand += value
			}
		}
	}
	_ = a.grandLabel.SetText(fmtNum(grand))
}

func (a *calendarApp) updateAllTotals() {
	for r := range a.rows {
		total := 0.0
		for _, entry := range a.rows[r].entries {
			if value, ok := parseHours(entry.Text()); ok {
				total += value
			}
		}
		_ = a.rows[r].total.SetText(fmtNum(total))
	}
	a.updateGrandTotal()
}

func (a *calendarApp) applyEmphasis() {
	calendarOn := a.mode == modeCalendar
	if calendarOn {
		_ = a.toggle.SetText("突出显示：日历")
		_ = a.pill.SetText("当前突出：日历（日期）")
		a.pill.SetBackground(a.style.brush("dayActiveBg"))
		a.pill.SetTextColor(a.style.color("dayActiveFg"))
	} else {
		_ = a.toggle.SetText("突出显示：工时")
		_ = a.pill.SetText("当前突出：工时（可编辑）")
		a.pill.SetBackground(a.style.brush("totalBg"))
		a.pill.SetTextColor(a.style.color("totalFg"))
	}

	for _, row := range a.rows {
		for _, cell := range row.days {
			if calendarOn {
				fg := a.style.color("dayActiveFg")
				if cell.weekend {
					fg = a.style.color("weekendFg")
				}
				bg := "dayActiveBg"
				if cell.today {
					bg = "todayBg"
				}
				cell.label.SetFont(a.style.fonts.dayActive)
				cell.label.SetTextColor(fg)
				cell.label.SetBackground(a.style.brush(bg))
			} else {
				fg := a.style.color("dayIdleFg")
				if cell.weekend {
					fg = a.style.color("weekendFg")
				}
				bg := "panel"
				if cell.today {
					bg = "todayBg"
				}
				cell.label.SetFont(a.style.fonts.dayIdle)
				cell.label.SetTextColor(fg)
				cell.label.SetBackground(a.style.brush(bg))
			}
		}
		for _, entry := range row.entries {
			a.restoreEntryStyle(entry)
		}
		if calendarOn {
			row.total.SetFont(a.style.fonts.totalIdle)
		} else {
			row.total.SetFont(a.style.fonts.totalActive)
		}
	}
	if a.grandLabel != nil {
		if calendarOn {
			a.grandLabel.SetFont(a.style.fonts.totalIdle)
		} else {
			a.grandLabel.SetFont(a.style.fonts.totalActive)
		}
	}
}

func (a *calendarApp) toggleMode() {
	if a.mode == modeCalendar {
		a.mode = modeHours
	} else {
		a.mode = modeCalendar
	}
	a.applyEmphasis()
}

func (a *calendarApp) copyTotals() {
	lines := make([]string, 0, len(a.rows))
	for _, row := range a.rows {
		lines = append(lines, row.total.Text())
	}
	text := strings.Join(lines, "\n")
	_ = walk.Clipboard().Clear()
	_ = walk.Clipboard().SetText(text)
	_ = a.copyBtn.SetText("已复制")
	a.copyBtn.SetBackground(a.style.brush("btn2Active"))
	time.AfterFunc(1200*time.Millisecond, func() {
		a.win.Synchronize(func() {
			_ = a.copyBtn.SetText("复制工时累计")
			a.copyBtn.SetBackground(a.style.brush("btn2Bg"))
		})
	})
}

func main() {
	app, err := newCalendarApp()
	if err != nil {
		walk.MsgBox(nil, "启动失败", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
		return
	}
	app.win.Show()
	app.win.Run()
}
