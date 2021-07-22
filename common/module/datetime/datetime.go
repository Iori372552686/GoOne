package datetime

import (
	"fmt"
	"sync"
	"time"
)

const (
	SECONDS_PER_MINUTE = 60
	MINUTES_PER_HOUR   = 60
	HOURS_PER_DAY      = 24
	SECONDS_PER_HOUR   = SECONDS_PER_MINUTE * MINUTES_PER_HOUR
	SECONDS_PER_DAY    = SECONDS_PER_HOUR * HOURS_PER_DAY
	MINUTES_PER_DAY    = MINUTES_PER_HOUR * HOURS_PER_DAY

	MS_PER_MINUTE = SECONDS_PER_MINUTE * 1000
	MS_PER_HOUR   = SECONDS_PER_HOUR * 1000
	MS_PER_DAY    = SECONDS_PER_DAY * 1000
	MS_PER_SECOND = 1000
)

var (
	gtime *Gtime
	once  sync.Once
)

type Gtime struct {
	TimeNow     time.Time
	AutoRefresh bool

	TimeOffSet int32
	lock       sync.RWMutex
}

func (t *Gtime) tick() {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.TimeNow = time.Now()
}

func getIns() *Gtime {
	once.Do(func() {
		gtime = &Gtime{
			TimeNow:     time.Now(),
			AutoRefresh: false,
		}
	})

	if gtime.AutoRefresh {
		gtime.tick()
	}
	return gtime
}

func SetAutoRefresh() {
	getIns().lock.Lock()
	defer getIns().lock.Unlock()

	getIns().AutoRefresh = true
}

func SetTimeOffset(offset int32) {
	getIns().lock.Lock()
	defer getIns().lock.Unlock()

	getIns().TimeOffSet = offset
}

func Tick() {
	getIns().tick()
}

func NowT() time.Time {
	getIns().lock.RLock()
	defer getIns().lock.RUnlock()

	t := getIns().TimeNow
	return t
}

func Now() int32 {
	getIns().lock.RLock()
	defer getIns().lock.RUnlock()

	return int32(getIns().TimeNow.Unix()) + getIns().TimeOffSet
}

func NowNoOffset() int32 {
	getIns().lock.RLock()
	defer getIns().lock.RUnlock()

	return int32(getIns().TimeNow.Unix())
}

func NowMs() int64 {
	getIns().lock.RLock()
	defer getIns().lock.RUnlock()

	return getIns().TimeNow.UnixNano()/1000000 + int64(getIns().TimeOffSet*MS_PER_SECOND)
}

func BeginTimeOfToday() int32 {
	now := Now()
	left := now % SECONDS_PER_DAY
	return now - left
}

// YYYYMMDD
func GetData() string {
	now := NowT()
	return now.Format("20060102")
}

// YYYYMMDDHHMM
func GetDataHM() string {
	now := NowT()
	return now.Format("200601021504")
}

func GetHourMinute() (int, int) {
	t1 := time.Unix(int64(Now()), 0)
	return t1.Hour(), t1.Minute()
}

func GetHourMinuteForTime(now int32) (int, int) {
	t1 := time.Unix(int64(now), 0)
	return t1.Hour(), t1.Minute()
}

func IsSameMinute(t1, t2 int32) bool {
	return t1/SECONDS_PER_MINUTE == t2/SECONDS_PER_MINUTE
}

func IsSameHour(t1, t2 int32) bool {
	return t1/SECONDS_PER_HOUR == t2/SECONDS_PER_HOUR
}

// 相差的天数，要考虑时区
func DifferenceDays(t1, t2 int32) int {
	if t1 > t2 {
		t1, t2 = t2, t1
	}

	d := (t2 - t1) / SECONDS_PER_DAY
	t := t1 + SECONDS_PER_DAY*d
	if !IsSameDay(t, t2) {
		d++
	}

	return int(d)
}

func IsSameDay(t1, t2 int32) bool {
	time1 := time.Unix(int64(t1), 0)
	time2 := time.Unix(int64(t2), 0)
	return time1.YearDay() == time2.YearDay() && time1.Year() == time2.Year()
}

func CalcIsSameDay(t1, t2 int64) bool {
	time1 := time.Unix(t1, 0)
	time2 := time.Unix(t2, 0)
	return time1.YearDay() == time2.YearDay() && time1.Year() == time2.Year()
}

func IsSameDayByDayBeginHour(t1, t2 int32, dayBeginTime int) bool {
	time1 := time.Unix(int64(t1), 0)
	time2 := time.Unix(int64(t2), 0)
	fmt.Println(time2.Hour())
	return IsSameDay(t1, t2) && time1.Hour() < dayBeginTime && time2.Hour() >= dayBeginTime
}

func IsSameWeek(t1, t2 int32) bool {
	lt1 := int64(t1)
	lt2 := int64(t2)
	tt1 := time.Unix(lt1, 0)
	tt2 := time.Unix(lt2, 0)
	y1, w1 := tt1.ISOWeek()
	y2, w2 := tt2.ISOWeek()
	return y1 == y2 && w1 == w2
}

func IsSameMonth(t1, t2 int32) bool {
	lt1 := int64(t1)
	lt2 := int64(t2)
	tt1 := time.Unix(lt1, 0)
	tt2 := time.Unix(lt2, 0)
	return tt1.Year() == tt2.Year() && tt1.Month() == tt2.Month()
}

func GetDayOfMonth(t1 int32) int32 {
	lt1 := int64(t1)
	tt1 := time.Unix(lt1, 0)
	_, _, day := tt1.Date()
	return int32(day)
}

func GetDayOfWeek(t1 int32) int32 {
	lt1 := int64(t1)
	tt1 := time.Unix(lt1, 0)
	return int32(tt1.Weekday())
}

// 时间区间，一般用来判断是否活动开启时间
func InTimeRange(bSecond, eSecond int) bool {
	now := time.Unix(int64(Now()), 0)
	t := now.Hour()*SECONDS_PER_HOUR + now.Minute()*SECONDS_PER_MINUTE + now.Second()
	//logger.Infof("now hour %d, now minute %d, t %d", now.Hour(), now.Minute(), t)
	return t >= bSecond && t <= eSecond
}

// 获取当天XX：XX分的时间
func GetTodayAssignTime(t time.Time, hour, minute int) time.Time {
	y, m, d := t.Date()

	return time.Date(y, m, d, hour, minute, 0, 0, time.Local)
}
