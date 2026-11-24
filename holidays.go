package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var holidayURLs = []string{
	"https://fastly.jsdelivr.net/gh/NateScarlet/holiday-cn@master/%d.json",
	"https://cdn.jsdelivr.net/gh/NateScarlet/holiday-cn@master/%d.json",
	"https://raw.githubusercontent.com/NateScarlet/holiday-cn/master/%d.json",
}

type builtinHolidays struct {
	Off  []string
	Work []string
}

var builtinYearData = map[int]builtinHolidays{
	2026: {
		Off: []string{
			"01-01", "01-02", "01-03",
			"02-15", "02-16", "02-17", "02-18", "02-19", "02-20", "02-21", "02-22", "02-23",
			"04-04", "04-05", "04-06",
			"05-01", "05-02", "05-03", "05-04", "05-05",
			"06-19", "06-20", "06-21",
			"09-25", "09-26", "09-27",
			"10-01", "10-02", "10-03", "10-04", "10-05", "10-06", "10-07",
		},
		Work: []string{"01-04", "02-14", "02-28", "05-09", "09-20", "10-10"},
	},
	2025: {
		Off: []string{
			"01-01",
			"01-28", "01-29", "01-30", "01-31", "02-01", "02-02", "02-03", "02-04",
			"04-04", "04-05", "04-06",
			"05-01", "05-02", "05-03", "05-04", "05-05", "05-31", "06-01", "06-02",
			"10-01", "10-02", "10-03", "10-04", "10-05", "10-06", "10-07", "10-08",
		},
		Work: []string{"01-26", "02-08", "04-27", "05-10", "09-28", "10-11"},
	},
}

type holidayData struct {
	Off  map[string]struct{}
	Work map[string]struct{}
}

func newHolidayData() holidayData {
	return holidayData{
		Off:  make(map[string]struct{}),
		Work: make(map[string]struct{}),
	}
}

func (d holidayData) has(m map[string]struct{}, date time.Time) bool {
	_, ok := m[date.Format("2006-01-02")]
	return ok
}

type holidayAPIItem struct {
	Date     string `json:"date"`
	IsOffDay *bool  `json:"isOffDay"`
}

type holidayAPIPayload struct {
	Year int              `json:"year"`
	Days []holidayAPIItem `json:"days"`
}

type holidayCacheEntry struct {
	Off  []string `json:"off"`
	Work []string `json:"work"`
}

func cacheFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".work_calendar_holiday_cache.json"
	}
	return filepath.Join(home, ".work_calendar_holiday_cache.json")
}

func fetchYearHolidays(year int) *holidayData {
	client := &http.Client{Timeout: 10 * time.Second}
	for _, tpl := range holidayURLs {
		data, ok := fetchOneHolidayURL(client, tpl, year)
		if ok {
			return data
		}
	}
	return nil
}

func fetchOneHolidayURL(client *http.Client, tpl string, year int) (*holidayData, bool) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(tpl, year), nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var payload holidayAPIPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, false
	}
	if payload.Year != year {
		return nil, false
	}
	data := newHolidayData()
	for _, item := range payload.Days {
		if item.IsOffDay != nil && !*item.IsOffDay {
			data.Work[item.Date] = struct{}{}
		} else {
			data.Off[item.Date] = struct{}{}
		}
	}
	if len(data.Off) == 0 && len(data.Work) == 0 {
		return nil, false
	}
	return &data, true
}

func loadDiskCache(year int) *holidayData {
	raw, err := os.ReadFile(cacheFilePath())
	if err != nil {
		return nil
	}
	var cache map[string]holidayCacheEntry
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil
	}
	item, ok := cache[strconv.Itoa(year)]
	if !ok {
		return nil
	}
	data := newHolidayData()
	for _, d := range item.Off {
		data.Off[d] = struct{}{}
	}
	for _, d := range item.Work {
		data.Work[d] = struct{}{}
	}
	return &data
}

func saveDiskCache(year int, data *holidayData) {
	path := cacheFilePath()
	cache := make(map[string]holidayCacheEntry)
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &cache)
	}
	off := make([]string, 0, len(data.Off))
	for d := range data.Off {
		off = append(off, d)
	}
	sort.Strings(off)
	work := make([]string, 0, len(data.Work))
	for d := range data.Work {
		work = append(work, d)
	}
	sort.Strings(work)
	cache[strconv.Itoa(year)] = holidayCacheEntry{Off: off, Work: work}
	raw, err := json.MarshalIndent(cache, "", " ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0644)
}

func builtinHolidayData(year int) *holidayData {
	item, ok := builtinYearData[year]
	if !ok {
		return nil
	}
	data := newHolidayData()
	toFull := func(mds []string) map[string]struct{} {
		out := make(map[string]struct{})
		for _, md := range mds {
			parts := strings.Split(md, "-")
			if len(parts) != 2 {
				continue
			}
			out[fmt.Sprintf("%04d-%s-%s", year, parts[0], parts[1])] = struct{}{}
		}
		return out
	}
	data.Off = toFull(item.Off)
	data.Work = toFull(item.Work)
	return &data
}

type holidayProvider struct {
	cache  map[int]*holidayData
	source string
}

func newHolidayProvider() *holidayProvider {
	return &holidayProvider{cache: make(map[int]*holidayData)}
}

func (p *holidayProvider) get(year int) *holidayData {
	if data, ok := p.cache[year]; ok {
		p.source = "内存缓存"
		return data
	}

	data := fetchYearHolidays(year)
	p.source = "在线接口"
	if data == nil {
		data = loadDiskCache(year)
		p.source = "本地缓存文件"
	}
	if data == nil {
		data = builtinHolidayData(year)
		p.source = "内置数据"
	}
	if data == nil {
		data = newHolidayDataPtr()
		p.source = "无数据，仅按周末判断"
	} else {
		saveDiskCache(year, data)
	}
	p.cache[year] = data
	return data
}

func newHolidayDataPtr() *holidayData {
	d := newHolidayData()
	return &d
}

func isWorkday(date time.Time, data *holidayData) bool {
	if data != nil {
		if data.has(data.Work, date) {
			return true
		}
		if data.has(data.Off, date) {
			return false
		}
	}
	wd := date.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}
