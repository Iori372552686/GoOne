package ssdb

import (
	"math"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/seefan/gossdb/v2"
	"github.com/seefan/gossdb/v2/conf"
)

func main() {
	p, err := gossdb.NewPool(&conf.Config{
		Host:        "127.0.0.1",
		Port:        8888,
		MaxWaitSize: 10000,
		PoolSize:    5,
		MinPoolSize: 5,
		MaxPoolSize: 20,
		AutoClose:   true,
		//Password:     "vdsfsfafapaddssrd#@Ddfasfdsfedssdfsdfsd",
		HealthSecond: 10,
	})
	if err != nil {
		panic(err)
	}
	defer p.Close()

	var wait sync.WaitGroup
	for i := 0; i < 7; i++ {
		wait.Add(1)
		go func() {
			for k := 0; k < 100; k++ {
				for j := 0; j < 100; j++ {
					c, e := p.NewClient()

					if e != nil {
						println(e.Error())
					} else {
						time.Sleep(time.Millisecond * time.Duration(math.Round(10)))
						c.Close()
					}
				}
				//println(p.Info())
			}
			wait.Done()
		}()
	}
	wait.Wait()
	//bs := make([]byte, 1)
	//os.Stdin.Read(bs)
}
func testReadme() {
	err := gossdb.Start(&conf.Config{
		Host: "127.0.0.1",
		Port: 8888,
	})
	if err != nil {
		panic(err)
	}
	defer gossdb.Shutdown()
	c, err := gossdb.NewClient()
	if err != nil {
		panic(err)
	}
	defer c.Close()
	if v, err := c.Get("a"); err == nil {
		println(v.String())
	} else {
		println(err.Error())
	}
	if v, err := c.Get("b"); err == nil {
		println(v.String())
	} else {
		println(err.Error())
	}
	//打开连接池，使用默认配置,Host=127.0.0.1,Port=8888,AutoClose=true
	if err := gossdb.Start(); err != nil {
		panic(err)
	}
	//别忘了结束时关闭连接池，当然如果你没有关闭，ssdb也会因错误中断连接的
	defer gossdb.Shutdown()
	//使用连接，因为AutoClose为true，所以我们没有手工关闭连接
	//gossdb.Client()为无错误获取连接方式，所以可以在获取连接后直接调用其它操作函数，如果获取连接出错或是调用函数出错，都会返回err
	if v, err := gossdb.Client().Get("a"); err == nil {
		println(v.String())
	} else {
		println(err.Error())
	}
}
