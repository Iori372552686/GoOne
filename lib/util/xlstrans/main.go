package main

import (
	"bufio"
	"fmt"
	"github.com/tealeg/xlsx"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// 每个xlsx数据文件，会对应生成以下文件
//   . dataconfig_<struct_name>_conf.proto (数据格式)
//   . dataconfig_<struct_name>.data (数据)
//   . <struct_name>_mgr.go (数据访问代码)
// 最后，生成一个管理所有数据的代码文件
//   . all_data.go

var templateFilePath = "conf_mgr.go.template"
var targetGoDirPath = "./go/gen"
var targetProtoPath = "./proto"
var targetDataPath = "./data"

func openSheet(fileName string) *xlsx.Sheet {
	file, err := xlsx.OpenFile(fileName)
	if err != nil {
		fmt.Println("open xlsx error", fileName, err)
		return nil
	}

	if len(file.Sheet) <= 0 {
		fmt.Println("empty xlsx:", fileName)
		return nil
	}

	return file.Sheets[0]
}

func main() {
	f, err := os.Open("xls_list.txt")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	os.MkdirAll("./proto", 0644)
	os.MkdirAll("./data", 0644)
	os.MkdirAll("./go/gen", 0644)

	structNames := make([]string, 0)

	// 扫描列表文件
	var waitGroup sync.WaitGroup
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fmt.Println("[Process file]: ", line)
		if line[0] == '#' { // this line is commented
			continue
		}

		structName := strings.TrimSuffix(filepath.Base(line), filepath.Ext(line))
		structName = structName + "Conf"
		structNames = append(structNames, structName)

		// 每个文件起一个协程处理
		waitGroup.Add(1)
		go func(fileName string, structName string) {
			defer waitGroup.Done()
			sheet := openSheet(fileName)

			if sheet == nil {
				fmt.Println("open sheet error: ", fileName)
				return
			}

			// 解析字段
			var x2p XlsToPb
			fileDesc := ParseStruct(structName, sheet)

			// 生成proto文件
			x2p.Trans(fileDesc, sheet)

			// 加载proto，并解析数据，生成数据文件
			var x2d XlsToData
			x2d.Parse(fileDesc, sheet)

			// 生成对应管理代码
			GenGoMgrFile(templateFilePath, structName)

			// 对常亮表做特殊处理
			if strings.Contains(fileName, "Const.xlsx") {
				var x2c XlsToConst
				x2c.GenConstFile(sheet)
			}
			//功能开放表 为了好获得ID
			if strings.Contains(fileName, "SystemUnlock.xlsx") {
				var x2s XlsToSystemUnlock
				x2s.GenConstFile(sheet)
			}

		}(line, structName)
	}

	waitGroup.Wait()

	// 生辰管理所有配置的代码
	GenAllIncludeFile(structNames)
}
