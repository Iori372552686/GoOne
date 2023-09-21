package datetime

import (
	"math"
	"sync"
	"time"
)

var (
	gtime *Gtime
	once  sync.Once
)

/*
*  Gtime
*  @Description: time struct
 */
type Gtime struct {
	TimeNow     time.Time
	AutoRefresh bool

	TimeOffSet int32
	//lock       sync.RWMutex
}

/**
* @Description: tick
* @receiver: t
* @Author: Iori
**/
func (t *Gtime) tick() {
	//t.lock.Lock()
	//defer t.lock.Unlock()

	t.TimeNow = time.Now()
}

/**
* @Description: get ins
* @return: *Gtime
* @Author: Iori
**/
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

/**
* @Description: AutoRefresh flag
* @Author: Iori
**/
func SetAutoRefresh() {
	//getIns().lock.Lock()
	//defer getIns().lock.Unlock()

	getIns().AutoRefresh = true
}

/**
* @Description: set Offset
* @param: offset
* @Author: Iori
**/
func SetTimeOffset(offset int32) {
	//getIns().lock.Lock()
	//defer getIns().lock.Unlock()

	getIns().TimeOffSet = offset
}

/**
* @Description: 外部tick
* @Author: Iori
**/
func Tick() {
	getIns().tick()
}

/**
* @Description: 获取当前now time
* @return: time.Time
* @Author: Iori
**/
func NowT() time.Time {
	//getIns().lock.RLock()
	//defer getIns().lock.RUnlock()

	t := getIns().TimeNow
	return t
}

/**
* @Description: 获取当前时间 now，返回int32秒，支持OffSet
* @return: int32
* @Author: Iori
**/
func Now() int32 {
	//getIns().lock.RLock()
	//defer getIns().lock.RUnlock()

	return int32(getIns().TimeNow.Unix()) + getIns().TimeOffSet
}

/**
* @Description:  获取当前时间 now，不带OffSet
* @return: int32
* @Author: Iori
**/
func NowNoOffset() int32 {
	//getIns().lock.RLock()
	//defer getIns().lock.RUnlock()

	return int32(getIns().TimeNow.Unix())
}

/**
* @Description: 获取当前时间now，返回int64秒，支持OffSet
* @return: int64
* @Author: Iori
**/
func NowInt64() int64 {
	//getIns().lock.RLock()
	//defer getIns().lock.RUnlock()

	return getIns().TimeNow.Unix() + int64(getIns().TimeOffSet)
}

/**
* @Description: 获取当前时间now，返回int64毫秒，支持OffSet
* @return: int64
* @Author: Iori
**/
func NowMs() int64 {
	//getIns().lock.RLock()
	//defer getIns().lock.RUnlock()

	return getIns().TimeNow.UnixNano()/1000000 + int64(getIns().TimeOffSet*MS_PER_SECOND)
}

/**
* @Description: 获取当前时间now，返回int64纳秒，支持OffSet
* @return: int64
* @Author: Iori
**/
func NowUs() int64 {
	//getIns().lock.RLock()
	//defer getIns().lock.RUnlock()

	return getIns().TimeNow.UnixNano()/1000 + int64(getIns().TimeOffSet*MS_PER_SECOND*1000)
}

/**
* @Description: 获取今日当前的秒数
* @return: int32
* @Author: Iori
**/
func BeginTimeOfToday() int32 {
	now := Now()
	left := now % SECONDS_PER_DAY
	return now - left
}

/**
* @Description: 获取当前日期格式 YYYY-MM-DD
* @return: string
* @Author: Iori
**/
func GetData() string {
	now := NowT()
	return now.Format("2006-01-02")
}

/**
* @Description: 获取当前日期格式 YYYY-MM-DD HH:MM:SS
* @return: string
* @Author: Iori
**/
func GetDataHMS() string {
	now := NowT()
	return now.Format("2006-01-02 15:04:16")
}

/**
* @Description: 获取小时，分钟
* @return: int
* @return: int
* @Author: Iori
**/
func GetHourMinute() (int, int) {
	t1 := time.Unix(int64(Now()), 0)
	return t1.Hour(), t1.Minute()
}

/**
* @Description: 根据传入time 获取小时分钟
* @param: now time
* @return: int
* @return: int
* @Author: Iori
**/
func GetHourMinuteForTime(now int32) (int, int) {
	t1 := time.Unix(int64(now), 0)
	return t1.Hour(), t1.Minute()
}

/**
* @Description: 是否同一分钟,根据秒来计算
* @param: t1
* @param: t2
* @return: bool
* @Author: Iori
**/
func IsSameMinuteBySec(t1, t2 int64) bool {
	return t1/MS_PER_MINUTE == t2/MS_PER_MINUTE
}

/**
* @Description: 是否同一分钟
* @param: t1
* @param: t2
* @return: bool
**/
func IsSameMinute(t1, t2 int64) bool {
	time1 := time.Unix(t1, 0)
	time2 := time.Unix(t2, 0)
	return IsSameDay(t1, t2) && time1.Hour() == time2.Hour() && time1.Minute() == time2.Minute()
}

/**
* @Description: 是否同一小时，根据秒来计算
* @param: t1
* @param: t2
* @return: bool
* @Author: Iori
**/
func IsSameHourBySec(t1, t2 int64) bool {
	return t1/SECONDS_PER_HOUR == t2/SECONDS_PER_HOUR
}

/**
* @Description: 是否同一小时
* @param: t1
* @param: t2
* @return: bool
* @Author: Iori
**/
func IsSameHour(t1, t2 int64) bool {
	time1 := time.Unix(t1, 0)
	time2 := time.Unix(t2, 0)
	return IsSameDay(t1, t2) && time1.Hour() == time2.Hour()
}

/**
* @Description: 计算相差的天数
* @param: t1
* @param: t2
* @return: int
* @Author: Iori
**/
func HowDiffDays(t1, t2 int64) int32 {
	if t1 > t2 {
		t1, t2 = t2, t1
	}

	d := (t2 - t1) / SECONDS_PER_DAY
	t := t1 + SECONDS_PER_DAY*d
	if !IsSameDay(t, t2) {
		d++
	}

	return int32(d)
}

/**
* @Description: 是否同一天
* @param: t1
* @param: t2
* @return: bool
* @Author: Iori
**/
func IsSameDay(t1, t2 int64) bool {
	time1 := time.Unix(t1, 0)
	time2 := time.Unix(t2, 0)
	return time1.YearDay() == time2.YearDay() && time1.Year() == time2.Year()
}

/**
* @Description: 是否已到开始的小时时间
* @param: t1
* @param: t2
* @param: dayBeginTime
* @return: bool
* @Author: Iori
**/
func IsSameDayByDayBeginHour(t1, t2 int64, dayBeginTime int) bool {
	time1 := time.Unix(t1, 0)
	time2 := time.Unix(t2, 0)
	return IsSameDay(t1, t2) && time1.Hour() < dayBeginTime && time2.Hour() >= dayBeginTime
}

/**
* @Description: 是否同一周
* @param: t1
* @param: t2
* @return: bool
* @Author: Iori
**/
func IsSameWeek(t1, t2 int64) bool {
	y1, w1 := time.Unix(t1, 0).ISOWeek()
	y2, w2 := time.Unix(t2, 0).ISOWeek()
	return y1 == y2 && w1 == w2
}

/**
* @Description: 是否同一月
* @param: t1
* @param: t2
* @return: bool
* @Author: Iori
**/
func IsSameMonth(t1, t2 int64) bool {
	tt1 := time.Unix(t1, 0)
	tt2 := time.Unix(t2, 0)
	return tt1.Year() == tt2.Year() && tt1.Month() == tt2.Month()
}

/**
* @Description: 是否同一年
* @param: t1
* @param: t2
* @return: bool
* @Author: Iori
**/
func IsSameYear(t1, t2 int64) bool {
	tt1 := time.Unix(t1, 0)
	tt2 := time.Unix(t2, 0)
	return tt1.Year() == tt2.Year()
}

/**
* @Description: 差多少分钟
* @param: t1
* @param: t2
* @return: int32
* @Author: Iori
**/
func HowDiffMin(t1, t2 int64) int32 {
	if t1 > t2 {
		t1, t2 = t2, t1
	}

	num := (t2 - t1) / SECONDS_PER_MINUTE
	t := t1 + SECONDS_PER_MINUTE*num
	if !IsSameMinute(t, t2) {
		num++
	}

	return int32(num)
}

/**
* @Description:  差多少小时
* @param: t1
* @param: t2
* @return: int32
* @Author: Iori
**/
func HowDiffHour(t1, t2 int64) int32 {
	if t1 > t2 {
		t1, t2 = t2, t1
	}

	num := (t2 - t1) / SECONDS_PER_HOUR
	t := t1 + SECONDS_PER_HOUR*num
	if !IsSameHour(t, t2) {
		num++
	}

	return int32(num)
}

/**
* @Description:  差多少周
* @param: t1
* @param: t2
* @return: int32
* @Author: Iori
**/
func HowDiffWeek(t1, t2 int64) int32 {
	if t1 > t2 {
		t1, t2 = t2, t1
	}

	num := (t2 - t1) / SECONDS_PER_WEEK
	t := t1 + SECONDS_PER_WEEK*num
	if !IsSameWeek(t, t2) {
		num++
	}

	return int32(num)
}

/**
* @Description:  差多少月
* @param: t1
* @param: t2
* @return: int32
* @Author: Iori
**/
func HowDiffMonth(t1, t2 int64) int32 {
	tt1 := time.Unix(t1, 0)
	tt2 := time.Unix(t2, 0)

	return int32(math.Abs(float64((tt1.Year()-tt2.Year())*12 + (int(tt1.Month()) - int(tt2.Month())))))
}

/**
* @Description:  差多少年
* @param: t1
* @param: t2
* @return: int32
* @Author: Iori
**/
func HowDiffYear(t1, t2 int64) int32 {
	tt1 := time.Unix(t1, 0)
	tt2 := time.Unix(t2, 0)

	return int32(math.Abs(float64(tt1.Year() - tt2.Year())))
}

/**
* @Description: 获取当前月的第几天
* @param: t1
* @return: int32
* @Author: Iori
**/
func GetDayOfMonth(t1 int32) int32 {
	tt1 := time.Unix(int64(t1), 0)
	_, _, day := tt1.Date()
	return int32(day)
}

/**
* @Description: 获取当前周的第几天
* @param: t1
* @return: int32
* @Author: Iori
**/
func GetDayOfWeek(t1 int32) int32 {
	lt1 := int64(t1)
	tt1 := time.Unix(lt1, 0)
	return int32(tt1.Weekday())
}

/**
* @Description: 时间区间，一般用来判断是否活动开启时间  -- 只限当地时区
* @param: bSecond
* @param: eSecond
* @return: bool
* @Author: Iori
**/
func InTimeRange(bSecond, eSecond int) bool {
	now := time.Unix(int64(Now()), 0)
	t := now.Hour()*SECONDS_PER_HOUR + now.Minute()*SECONDS_PER_MINUTE + now.Second()
	//logger.Infof("now hour %d, now minute %d, t %d", now.Hour(), now.Minute(), t)
	return t >= bSecond && t <= eSecond
}

/**
* @Description: 获取当天XX：XX分的时间
* @param: t
* @param: hour
* @param: minute
* @return: time.Time
* @Author: Iori
**/
func GetTodayAssignTime(t time.Time, hour, minute int) time.Time {
	y, m, d := t.Date()

	return time.Date(y, m, d, hour, minute, 0, 0, time.Local)
}
