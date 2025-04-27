package main

import (
	"flag"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/application"
)

func main() {
	flag.Parse()
	defer logger.Flush()

	application.Init(&AppSvrImpl{})
	application.Run()
}
