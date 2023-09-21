package sensitive_words

import (
	"GoOne/lib/api/logger"
	"bufio"
	"os"
	"strings"
)

type SensitiveCheck struct {
	sensitiveWords map[string]bool
	fastCheck      []uint8
	fastLength     []uint8
	charCheck      []bool
	endCheck       []bool
	maxWordLength  int
	minWordLength  int
}

func Init(fileName string) int {
	words := strings.Split(invalidWords, ",")
	for _, v := range words {
		invalidWord[v] = nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		logger.Fatalf("load sensitive word file error: %v", err)
		return -1
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := scanner.Text()
		l := len(word)
		if l == 0 {
			continue
		}
		set[word] = nil
	}
	addSensitiveToMap(set)
	return 0
}

// 敏感词汇转换为*
func ChangeSensitiveWords(txt string) (bool, string) {
	str := []rune(txt)
	nowMap := sensitiveWord
	start := -1
	tag := -1
	find := false
	for i := 0; i < len(str); i++ {
		if _, ok := invalidWord[(string(str[i]))]; ok {
			continue //如果是无效词汇直接跳过
		}
		if thisMap, ok := nowMap[string(str[i])].(map[string]interface{}); ok {
			//记录敏感词第一个文字的位置
			tag++
			if tag == 0 {
				start = i
			}
			//判断是否为敏感词的最后一个文字
			if isEnd, _ := thisMap["isEnd"].(bool); isEnd {
				//将敏感词的第一个文字到最后一个文字全部替换为“*”
				for y := start; y < i+1; y++ {
					str[y] = '*'
					find = true
				}
				//重置标志数据
				nowMap = sensitiveWord
				start = -1
				tag = -1

			} else { //不是最后一个，则将其包含的map赋值给nowMap
				nowMap = nowMap[string(str[i])].(map[string]interface{})
			}
		} else { //如果敏感词不是全匹配，则终止此敏感词查找。从开始位置的第二个文字继续判断
			if start != -1 {
				i = start + 1
			}
			//重置标志参数
			nowMap = sensitiveWord
			start = -1
			tag = -1
		}
	}

	return find, string(str)
}

/// privite

var sensitiveWord = make(map[string]interface{})
var set = make(map[string]interface{})

const invalidWords = " ,~,!,@,#,$,%,^,&,*,(,),_,-,+,=,?,<,>,.,—,，,。,/,\\,|,《,》,？,;,:,：,',‘,；,“,"

var invalidWord = make(map[string]interface{}) //无效词汇，不参与敏感词汇判断直接忽略

// 生成违禁词集合
func addSensitiveToMap(set map[string]interface{}) {
	for key := range set {
		str := []rune(key)
		nowMap := sensitiveWord
		for i := 0; i < len(str); i++ {
			if _, ok := nowMap[string(str[i])]; !ok { //如果该key不存在，
				thisMap := make(map[string]interface{})
				thisMap["isEnd"] = false
				nowMap[string(str[i])] = thisMap
				nowMap = thisMap
			} else {
				nowMap = nowMap[string(str[i])].(map[string]interface{})
			}
			if i == len(str)-1 {
				nowMap["isEnd"] = true
			}
		}

	}
}
