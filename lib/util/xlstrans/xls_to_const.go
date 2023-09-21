package main

import (
	"fmt"
	"github.com/tealeg/xlsx"
	"io/ioutil"
	"strings"
)

type XlsToConst struct {
	content string
}

func (c *XlsToConst) GenConstFile(sheet *xlsx.Sheet) {
	c.genHead()
	c.genFunc(sheet)
	c.writeFile()
}

func (c *XlsToConst) genHead() {
	c.content += `
package gamedata

var Const ConstType

type ConstType struct {
}
`
}

func (c *XlsToConst) genFunc(sheet *xlsx.Sheet) {
	for i := 4; i < len(sheet.Rows); i++ {
		id := strings.TrimSpace(sheet.Cell(i, 0).String())
		name := strings.TrimSpace(sheet.Cell(i, 1).String())
		if id == "" || name == "" {
			continue
		}

		c.content += "func (* ConstType) " + name + "()" + " int32 {\n	return int32(ConstConfMgr.GetOne(" + id + ").Value)\n}\n"
	}
}

func (c *XlsToConst) writeFile() {
	err := ioutil.WriteFile(targetGoDirPath+"/const.go", []byte(c.content), 0644)
	if err != nil {
		fmt.Println(err.Error())
	}
}
