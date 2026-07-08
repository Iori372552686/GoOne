package rest

import (
	_ "github.com/go-sql-driver/mysql"

	"html/template"
)

var restFuncMap = make(template.FuncMap)

func init() {
	restFuncMap["ctxpath"] = ctxpath
	restFuncMap["apiurl"] = apiurl
	restFuncMap["version"] = version
	restFuncMap["hello"] = hello
	restFuncMap["asset"] = asset
}

func asset() string {
	return ""
}

func GetFuncMap() template.FuncMap {
	return restFuncMap
}

func hello(d string) string {
	return "hello " + d
}

func ctxpath() string {
	return ""
}

func apiurl(uri string) string {
	return uri
}

func version() string {
	return "v1.1.0" //todo
}
