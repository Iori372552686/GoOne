package base

import (
	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
)

func Sub(a, b int) int {
	return a - b
}

func Add(a, b int) int {
	return a + b
}

func GetProtoName(name string) string {
	return name + ".proto"
}

func Ifelse[T any](flag bool, a, b T) T {
	if flag {
		return a
	}
	return b
}

func Prefix[T any](vals []T, pos int) []T {
	if pos < 0 || pos >= len(vals) {
		return nil
	}
	return vals[:pos]
}

func Suffix[T any](vals []T, pos int) []T {
	if pos < 0 || pos >= len(vals) {
		return nil
	}
	return vals[pos:]
}

func Save(path, filename string, buf []byte) error {
	fileName := filepath.Join(path, filename)
	// 创建目录
	if err := os.MkdirAll(filepath.Dir(fileName), os.FileMode(0777)); err != nil {
		return uerror.New(1, -1, "filename: %s, error: %v", fileName, err)
	}

	// 写入文件
	if err := ioutil.WriteFile(fileName, buf, os.FileMode(0666)); err != nil {
		return uerror.New(1, -1, "filename: %s, error: %v", fileName, err)
	}
	return nil
}

func SaveGo(path, filename string, buf []byte) error {
	result, err := format.Source(buf)
	if err != nil {
		Save("./", "gen_error.gen.go", buf)
		return uerror.New(1, -1, "格式化失败: %v", err)
	}
	return Save(path, filename, result)
}

// 遍历目录所有文件
func Glob(dir, pattern string, recursive bool) (rets []string, err error) {
	pre, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// 不深度迭代
		if !recursive && info.IsDir() && dir != path {
			return filepath.SkipDir
		}
		// 过滤目录
		if info.IsDir() {
			return nil
		}
		// 是否配置
		if pre.MatchString(path) {
			rets = append(rets, path)
		}
		return nil
	})
	return
}
