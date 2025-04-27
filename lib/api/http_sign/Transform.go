package http_sign

import (
	"net/url"
	"sort"
	"strings"
)

/**
 * @Description: 把URL字符串参数转为参数列表map
 * @Author Iori
 * @Date 2022-01-22 18:33:33
 * @param params
 * @return map[string]string
 **/
func UriParam2Map(params string) *map[string]string {
	requestParamMap := make(map[string]string)
	if params == "" {
		return &requestParamMap
	}

	arr := strings.Split(params, "&")

	for _, s := range arr {
		kvArr := strings.Split(s, "=")
		if len(kvArr) > 1 {
			requestParamMap[kvArr[0]] = kvArr[1]
		}
	}
	return &requestParamMap
}

/**
* @Description: 把参数列表map转为URL字符串参数，,外部使用
* @param: params
* @param: url_encode
* @return: string
* @Author: Iori
* @Date: 2022-02-17 10:01:06
**/
func MapParam2Uri(params *map[string]string, url_encode bool) string {
	if params == nil {
		return ""
	}
	return Map2uri(params, "", false, url_encode)
}

/**
* @Description: map转为URL字符串参数 ,内部使用
* @param: params
* @param: filter_field
* @param: need_sort
* @param: url_encode
* @return: string
* @Author: Iori
* @Date: 2022-02-17 10:12:58
**/
func Map2uri(params *map[string]string, filter_field string, need_sort, url_encode bool) string {
	if params == nil {
		return ""
	}
	var strParams string
	var keys []string

	for k, _ := range *params {
		if k != filter_field {
			keys = append(keys, k)
		}
	}

	//排序
	if need_sort {
		sort.Strings(keys)
	}

	//拼接
	for _, key := range keys {
		val := (*params)[key]
		//url 编码
		if url_encode {
			val = url.QueryEscape(val)
		}

		if val != "" {
			if len(strParams) > 0 {
				strParams = strParams + "&"
			}
			strParams = strParams + key + "=" + val
		}
	}

	return strParams
}
