package gamedata

import (
	"testing"
	"time"

	"github.com/go-zookeeper/zk"
)

func TestMain(m *testing.M) {
	if err := InitLocal("./data"); err != nil {
		panic(err)
	}

	m.Run()
}

func TestZK(t *testing.T) {
	zkConn, chanConnect, err := zk.Connect([]string{"192.168.50.11:2182"}, time.Second*30)
	t.Log(err, zkConn, chanConnect)
}
