package main

import (
	"fmt"
	"github.com/tealeg/xlsx"
	"io/ioutil"
	"strings"
)

type XlsToSystemUnlock struct {
	content string
}

func (c *XlsToSystemUnlock) GenConstFile(sheet *xlsx.Sheet) {
	c.genHead()
	c.genFunc(sheet)
	c.writeFile()
}

func (c *XlsToSystemUnlock) genHead() {
	c.content += `
package gamedata

var SystemUnlock SystemUnlockType

type SystemUnlockType struct {
}
`
}

func (c *XlsToSystemUnlock) genFunc(sheet *xlsx.Sheet) {
	for i := 4; i < len(sheet.Rows); i++ {
		id := strings.TrimSpace(sheet.Cell(i, 0).String())
		name := strings.TrimSpace(sheet.Cell(i, 1).String())
		if id == "" || name == "" {
			continue
		}

		c.content += "func (* SystemUnlockType) " + name + "FuncId()" + " int32 {\n	return int32(SystemUnlockConfMgr.GetOne(" + id + ").Id)\n}\n"
	}
}

func (c *XlsToSystemUnlock) writeFile() {
	err := ioutil.WriteFile(targetGoDirPath+"/system_unlock.go", []byte(c.content), 0644)
	if err != nil {
		fmt.Println(err.Error())
	}
}
