package main

import (
	"flag"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/application"
	"github.com/Iori372552686/GoOne/src/mainsvr"
)

func main() {
	flag.Parse()
	defer logger.Flush()

	application.Init(mainsvr.NewApp())
	application.Run()
}
