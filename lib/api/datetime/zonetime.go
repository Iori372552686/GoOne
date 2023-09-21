package datetime

import (
	"strings"
	"time"
)

/*
* @title:  判断是否在活动时间内,支持不同时区
* @description:  根据传入的时间，还有时区差，判断是否在活动时间内
* @param: 开始时间，结束时间，时区偏移
* @return: bool
* @author   Iori
 */
func InTimeRangeByZone(start_time, end_time int64, local_tz bool, utc_offset int32, country string) bool {
	zone_now := LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country)
	return zone_now >= start_time && zone_now < end_time
}

/*
* @title:  判断指定时间是否在活动时间内,支持不同时区
* @description:  根据传入的时间，还有时区差，判断是否在活动时间内
* @param: 开始时间，结束时间，时区偏移
* @return: bool
* @author   Iori
 */
func AppInTimeRangeByZone(start_time, end_time int64, local_tz bool, utc_offset int32, country string, app_time int64) bool {
	zone_now := LocalTimestamp(app_time, local_tz, utc_offset, country)
	return zone_now >= start_time && zone_now < end_time
}

/**
* @Description: 根据字符串获取时间戳
* @param: time_format
* @param: local_tz
* @param: utc_offset
* @return: int64
* @Author: Iori
**/
func ParseTimestamp(time_format string, local_tz bool, utc_offset int32, country string) int64 {
	time, _ := time.ParseInLocation("2006-01-02 15:04:05", time_format, time.UTC)
	return LocalTimestamp(time.Unix(), local_tz, utc_offset, country)
}

/**
* @Description: 对比活动时间差了多少秒
* @param: start_time
* @param: end_time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: int64
* @Author: Iori
**/
func DiffTimeRange(start_time, end_time int64, local_tz bool, utc_offset int32, country string) int64 {
	zone_now := LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country)
	if zone_now-start_time < 0 {
		return zone_now - start_time
	} else if zone_now-end_time > 0 {
		return zone_now - end_time
	}

	return 0
}

/**
* @Description: 获取偏移时间戳
* @param: tt
* @param: local_tz
* @param: utc_offset 偏移分钟
* @param: country
* @return: int64 时间戳
* @Author: Iori
**/
func LocalTimestamp(tt int64, local_tz bool, utc_offset int32, country string) int64 {
	if tt == 0 {
		tt = time.Now().Unix()
	}

	if !local_tz {
		if utc_offset == 0 {
			utc_offset = GetConfTimeOffset(country)
		}
		tt += int64(utc_offset * SECONDS_PER_MINUTE)
	}

	return tt
}

/**
* @Description: 判断是否同一小时，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset  偏移分钟
* @return: bool
* @Author: Iori
**/
func SameHour(t1 int64, local_tz bool, utc_offset int32, country string) bool {
	return IsSameHour(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断是否同一天，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func SameDay(t1 int64, local_tz bool, utc_offset int32, country string) bool {
	return IsSameDay(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断是否同一周，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func SameWeek(t1 int64, local_tz bool, utc_offset int32, country string) bool {
	return IsSameWeek(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断是否同一月，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func SameMonth(t1 int64, local_tz bool, utc_offset int32, country string) bool {
	return IsSameMonth(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断是否同一年，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func SameYear(t1 int64, local_tz bool, utc_offset int32, country string) bool {
	return IsSameYear(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断当前时间差几分，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func DiffMin(t1 int64, local_tz bool, utc_offset int32, country string) int32 {
	return HowDiffMin(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断当前时间差几小时，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func DiffHour(t1 int64, local_tz bool, utc_offset int32, country string) int32 {
	return HowDiffHour(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断当前时间差几天，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func DiffDay(t1 int64, local_tz bool, utc_offset int32, country string) int32 {
	return HowDiffDays(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断当前时间差几周，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func DiffWeek(t1 int64, local_tz bool, utc_offset int32, country string) int32 {
	return HowDiffWeek(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断当前时间差几月，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func DiffMonth(t1 int64, local_tz bool, utc_offset int32, country string) int32 {
	return HowDiffMonth(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/**
* @Description: 判断当前时间差几年，支持时区偏移
* @param: time
* @param: local_tz
* @param: utc_offset 偏移分钟
* @return: bool
* @Author: Iori
**/
func DiffYear(t1 int64, local_tz bool, utc_offset int32, country string) int32 {
	return HowDiffYear(t1, LocalTimestamp(time.Now().Unix(), local_tz, utc_offset, country))
}

/*
* @title:  获取时间字符串的时间戳
* @description:  根据时间字符串，得到时间戳,秒单位
* @param: 时间字符串
* @return: int64 时间戳
 */
func GetUtcTimeSec(time_str string) int64 {
	time, _ := time.ParseInLocation("2006-01-02 15:04:05", time_str, time.UTC)
	return time.Unix()
}

/*
* @title:  获取字符串时间对应时区的时间戳
* @description:  根据时间字符串，得到对应时区时间戳,秒单位
* @param: 时间字符串，时区偏移
* @return: int64 时间戳
 */
func GetZoneTimeSec(time_str string, diff int64) int64 {
	time, _ := time.ParseInLocation("2006-01-02 15:04:05", time_str, time.UTC)
	return time.Unix() + diff*SECONDS_PER_HOUR
}

/*
* @title:  活动对应时区的时间戳
* @description:  手动时区偏移，得到对应时区的时间戳
* @param: 时区偏移
* @return: int64 时间戳
 */
func GetTimeByZone(diff int64) int64 {
	return time.Now().Add(time.Duration(diff) * time.Hour).Unix()
}

/*
* @title:  活动对应时区的时间戳
* @description:  手动时区偏移，得到对应时区的时间戳
* @param: 时区偏移  支持分钟
* @return: int64 时间戳
 */
func GetTimeByDiff(diff int64) int64 {
	return time.Now().Add(time.Duration(diff) * time.Minute).Unix()
}

/*
* @title:  判断是否在活动时间内,支持不同时区
* @description:  根据字符串的活动时间，还有时区差，判断是否在活动时间内
* @param: 开始时间，结束时间，时区偏移
* @return: bool
* @author   Iori
 */
func InTimeRangeByStr(start_str, end_str string, diff int64) bool {
	start := GetUtcTimeSec(start_str)
	end := GetUtcTimeSec(end_str)
	zone_now := GetTimeByZone(diff)

	return zone_now >= start && zone_now < end
}

/*
* @title:  获取某个时区的当前日期
* @description:  YYYY-MM-DD 支持时区
* @param: diff 时区偏移 小时
* @return: string 日期
* @author   Iori
 */
func GetDateByZone(diff int32) string {
	return time.Now().Add(time.Duration(diff) * time.Hour).UTC().Format("2006-01-02")
}

/*
* @title:  获取某个时区的当前日期
* @description:  YYYY-MM-DD 支持时区
* @param: diff 时区偏移 分钟
* @return: string 日期
* @author   Iori
 */
func GetDateByDiff(diff int32) string {
	return time.Now().Add(time.Duration(diff) * time.Minute).UTC().Format("2006-01-02")
}

/*
* @title:  获取某个时区的当前年
* @description:  YYYY  支持时区
* @param: diff 时区偏移 分钟
* @return: string 日期
* @author   Iori
 */
func GetYearByDiff(diff int32) string {
	return time.Now().Add(time.Duration(diff) * time.Minute).UTC().Format("2006")
}

/*
* @title:  获取某个时区的当前年与月
* @description:  YYYY-MM  支持时区
* @param: diff 时区偏移 分钟
* @return: string 日期
* @author   Iori
 */
func GetYearMonthByDiff(diff int32) string {
	return time.Now().Add(time.Duration(diff) * time.Minute).UTC().Format("2006-01")
}

/*
* @title:  获取某个时区的当前第几周
* @description:  YYYY-MM  支持时区
* @param: diff 时区偏移 分钟
* @return: string 日期
* @author   Iori
 */
func GetYearWeekByDiff(diff int32) (int, int) {
	return time.Now().Add(time.Duration(diff) * time.Minute).UTC().ISOWeek()
}

/**
* @Description: 根据国家参数读取大区配置时间偏移值
* @param: country
* @return: int32
* @Author: Iori
**/
func GetConfTimeOffset(country string) int32 {
	if country == "" {
		return 0
	}

	return ZoneTimeOffset[strings.ToUpper(country)]
}
