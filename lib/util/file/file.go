package file

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

/**
* @Description: 检查指定路径是否为文件夹
* @param: name
* @return: bool
* @Author: Iori
* @Date: 2022-06-10 18:04:21
**/
func IsDir(name string) bool {
	if info, err := os.Stat(name); err == nil {
		return info.IsDir()
	}
	return false
}

/**
* @Description:   获取指定路径下的所有文件，只搜索当前路径，不进入下一级目录，可匹配后缀过滤（suffix为空则不过滤）
* @param: dir
* @param: suffix
* @return: files
* @return: err
* @Author: Iori
* @Date: 2022-06-10 18:04:58
**/
func ListAllFile(dir, suffix string) (files []string, err error) {
	files = []string{}

	_dir, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	suffix = strings.ToLower(suffix) //匹配后缀

	for _, _file := range _dir {
		if len(suffix) == 0 || strings.HasSuffix(strings.ToLower(_file.Name()), suffix) {
			//文件后缀匹配
			files = append(files, _file.Name())
		}
	}

	return files, nil
}

/**
* @Description:  获取指定路径下的所有子目录
* @param: dir
* @return: files
* @return: err
* @Author: Iori
* @Date: 2022-06-13 15:11:07
**/
func ListDir(dir string) (files []string, err error) {
	files = []string{}

	_dir, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, _file := range _dir {
		if _file.IsDir() {
			files = append(files, _file.Name())
		}
	}

	return files, nil
}

/**
* @Description: 删除所有的匹配文件
* @param: srcDir
* @param: dstDir
* @return: error
* @Author: Iori
* @Date: 2022-06-13 15:27:43
**/
func MatchRemoveAll(srcDir, dstDir string) error {
	if srcDir == "" || dstDir == "" {
		return errors.New("dir args err!")
	}

	if !IsDir(dstDir) {
		return errors.New("dst not dir!")
	}

	dstList, _ := ListAllFile(dstDir, "")
	srcList, _ := ListAllFile(srcDir, "")
	for _, srcfile := range srcList {
		for _, dstfile := range dstList {
			if srcfile == dstfile {
				src := path.Join(srcDir, srcfile)
				dst := path.Join(dstDir, dstfile)

				if IsDir(src) {
					go MatchRemoveAll(src, dst)
				} else {
					os.Remove(dst)
				}
			}
		}
	}

	return nil
}
