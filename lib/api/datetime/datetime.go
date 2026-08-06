// Package datetime 提供进程级缓存的时间快照，避免业务热路径每次都调用
// time.Now() 的系统调用开销，并保证同一"逻辑帧"内所有调用方读到一致的时间值。
//
// 设计要点：
//   - 缓存值由外部周期性调用 Tick() 刷新（推荐通过 scheduler.DefaultDateTimeTick()
//     注册为 runtime.Component，默认 100ms 周期）。
//   - 内部用 atomic.Pointer[time.Time] / atomic.Int32 存储，消除高频读写下的
//     数据竞争（旧实现裸字段 + 注释掉的锁在 -race 下必报警）。
//   - 缓存精度 = 刷新周期（默认 ±100ms）。需要更高精度的单点调用应直接用
//     time.Now()，不要污染全局缓存。
//   - 保持叶子包：仅 import 标准库，不依赖 logger/scheduler，避免循环依赖。
package datetime

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultTickInterval 是 datetime 缓存的默认刷新周期。100ms 在"逻辑帧时间一致性"
// 与"系统调用开销"之间取平衡；可被 SetTickInterval 在组件启动前覆盖。
const DefaultTickInterval = 100 * time.Millisecond

var (
	// now 缓存的当前时间快照。atomic.Pointer 保证多 goroutine 读与单 goroutine
	// 写（Tick）的无锁安全。初始化为进程启动时刻，避免首次读取拿到零值。
	now atomic.Pointer[time.Time]

	// offset 秒级时间偏移（用于跨时区逻辑）。原子读写。
	offset atomic.Int32

	// tickInterval 被 scheduler.DefaultDateTimeTick 读取；SetTickInterval 可覆盖。
	// 仅在启动前写入，启动后并发读取，用 atomic.Pointer 持有避免 race。
	tickInterval atomic.Pointer[time.Duration]

	// once 保护初始快照与默认 tickInterval 的赋值，避免 init 顺序问题。
	once sync.Once
)

func init() {
	t := time.Now()
	now.Store(&t)
	d := DefaultTickInterval
	tickInterval.Store(&d)
}

// ensureInit 兜底初始化（覆盖被外部包跳过 init 的极端场景，如某些测试工具链）。
func ensureInit() {
	once.Do(func() {
		if now.Load() == nil {
			t := time.Now()
			now.Store(&t)
		}
		if tickInterval.Load() == nil {
			d := DefaultTickInterval
			tickInterval.Store(&d)
		}
	})
}

// SetTickInterval 设置默认刷新周期，供 scheduler.DefaultDateTimeTick() 引用。
// 必须在组件 Start 之前调用（启动后修改无效，因为 Task 已用旧值构造）。
// 传入非正值被忽略。
func SetTickInterval(d time.Duration) {
	ensureInit()
	if d <= 0 {
		return
	}
	tickInterval.Store(&d)
}

// TickInterval 返回当前生效的默认刷新周期。
func TickInterval() time.Duration {
	ensureInit()
	return *tickInterval.Load()
}

// Tick 刷新缓存快照一次。由外部周期驱动（推荐 scheduler.DefaultDateTimeTick）。
// 幂等、线程安全；调用方无需加锁。
func Tick() {
	t := time.Now()
	now.Store(&t)
}

// SetTimeOffset 设置秒级偏移（用于跨时区业务判定）。
func SetTimeOffset(off int32) {
	offset.Store(off)
}

// TimeOffset 返回当前秒级偏移。
func TimeOffset() int32 { return offset.Load() }

// snapshot 返回当前缓存时间（内部统一出口，便于将来加监控）。
func snapshot() time.Time {
	return *now.Load()
}

// -----------------------------------------------------------------------------
// 以下为对外稳定 API：签名与历史版本保持一致，仅内部存储由裸字段换成 atomic。
// 不再有 AutoRefresh / SetAutoRefresh（旧"每次读都 time.Now()"的退化路径，
// 与缓存初衷矛盾且零调用者，已删除）。
// -----------------------------------------------------------------------------

// NowT 获取当前缓存时间（精度 = 刷新周期）。
func NowT() time.Time { return snapshot() }

// Now 获取当前时间秒（int32），含 Offset。
func Now() int32 { return int32(snapshot().Unix()) + offset.Load() }

// NowNoOffset 获取当前时间秒（int32），不含 Offset。
func NowNoOffset() int32 { return int32(snapshot().Unix()) }

// NowInt64 获取当前时间秒（int64），含 Offset。
func NowInt64() int64 { return snapshot().Unix() + int64(offset.Load()) }

// NowMs 获取当前时间毫秒（int64），含 Offset。
func NowMs() int64 {
	return snapshot().UnixNano()/1e6 + int64(offset.Load())*int64(MS_PER_SECOND)
}

// NowUs 获取当前时间微秒（int64），含 Offset。
func NowUs() int64 {
	return snapshot().UnixNano()/1e3 + int64(offset.Load())*int64(MS_PER_SECOND*1000)
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
	return snapshot().Format("2006-01-02")
}

/**
 * @Description: 获取当前日期格式 YYYY-MM-DD HH:MM:SS
 * @return: string
 * @Author: Iori
 **/
func GetDataHMS() string {
	return snapshot().Format("2006-01-02 15:04:05")
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
