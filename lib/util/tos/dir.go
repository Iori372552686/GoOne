package tos

import (
	"errors"
	"io/ioutil"
	"os"
)

func ListDirFileName(dirPth string) (files []string, dirs []string, err error) {
	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return
	}
	for _, fi := range dir {
		if fi.IsDir() {
			dirs = append(dirs, fi.Name())
		} else {
			files = append(files, fi.Name())
		}
	}
	return
}

// ListDir 列出目录下所有文件（带完整路径）
func ListDir(dirPth string) (files []string, dirs []string, err error) {
	return ListDirFullPath(dirPth)
}

// ListDirFullPath 列出目录下所有文件（带完整路径）
func ListDirFullPath(dirPth string) (files []string, dirs []string, err error) {
	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return
	}
	PthSep := string(os.PathSeparator)

	for _, fi := range dir {
		if fi.IsDir() {
			dirs = append(dirs, dirPth+PthSep+fi.Name())
		} else {
			files = append(files, dirPth+PthSep+fi.Name())
		}
	}
	return
}

func IsFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return false
}
