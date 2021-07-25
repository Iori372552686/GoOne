/// 生成go的解析代码

package main

import (
	"fmt"
	"github.com/iancoleman/strcase"
	"io/ioutil"
	"os"
	"strings"
)

func GenGoMgrFile(templateFilePath string, structName string) {
	file, err := os.Open(templateFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	dataFileName := "dataconfig_" + structName + ".conf"

	str := string(content)
	str = strings.Replace(str, "{STRUCT_NAME}", strcase.ToCamel(structName),-1)
	str = strings.Replace(str, "{DATA_FILE_NAME}", dataFileName, -1)

	data := []byte(str)
	err = ioutil.WriteFile(targetGoDirPath + "/" + "mgr_" + structName + ".go", data, 0644)
	if err != nil {
		fmt.Println(err.Error())
	}

}

func GenAllIncludeFile(structNames []string) {
	str := ""
	str += "package gamedata\n\n"
	str += "import \"sync\"\n\n"

	for _, v := range structNames {
		str += "var " + strcase.ToCamel(v) + "Mgr " + strcase.ToCamel(v) + "MgrType\n"
	}

	str += "\nvar packageLocker  sync.RWMutex\n"

	str += "\nfunc GameDataInit(basePath string) int {\n"
	str += "	packageLocker.Lock()\n"
	str += "	defer packageLocker.Unlock()\n\n"
	for _, v := range structNames {
		str += "    " + strcase.ToCamel(v) + "Mgr" + ".InitFromFile(basePath)\n"
	}

	str += `
    return 0
}

func RLock() {
    packageLocker.RLock()
}

func RUnlock() {
    packageLocker.RUnlock()
}`

	err := ioutil.WriteFile(targetGoDirPath + "/all_data.go", []byte(str), 0644 )
	if err != nil {
		fmt.Println(err.Error())
	}
}
