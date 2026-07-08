package orm

import (
	"fmt"
	"log"
	"os"
	"testing"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

var engine *xorm.Engine

// TestMain is integration-flavoured: it requires a local MySQL. When MySQL is
// unavailable (e.g. CI with -short), all tests in this package are skipped.
func TestMain(m *testing.M) {
	cnn, err := xorm.NewEngine("mysql", "root:123456@tcp(127.0.0.1:3306)/testdb?charset=utf8")
	if err != nil {
		log.Printf("skip xorm integration tests: %v", err)
		os.Exit(0)
	}

	if err := cnn.Ping(); err != nil {
		log.Printf("skip xorm integration tests, mysql unavailable: %v", err)
		os.Exit(0)
	}

	if err := cnn.Sync2(new(g1_protocol.MysqlTexasRoomInfo)); err != nil {
		log.Printf("skip xorm integration tests, sync failed: %v", err)
		os.Exit(0)
	}

	engine = cnn
	os.Exit(m.Run())
}

func TestXorm(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; requires local mysql")
	}

	item := &g1_protocol.MysqlTexasRoomInfo{
		RoomId:  1,
		TableId: 1,
	}

	affected, err := engine.Insert(item)
	if err != nil {
		t.Log(affected, err)
		return
	}
}

// User 定义用户结构体，对应数据库中的表
type User struct {
	ID   int64  `xorm:"pk autoincr"`
	Name string `xorm:"varchar(100)"`
	Age  int    `xorm:"int"`
}

func TestUser(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; requires local mysql")
	}

	// 同步结构体到数据库表结构
	if err := engine.Sync2(new(User)); err != nil {
		t.Fatalf("同步表结构失败: %v", err)
	}

	// 插入数据
	user := &User{Name: "Alice", Age: 25}
	affected, err := engine.Insert(user)
	if err != nil {
		t.Fatalf("插入数据失败: %v", err)
	}
	fmt.Printf("插入 %d 条记录\n", affected)

	// 查询数据
	var users []User
	err = engine.Find(&users)
	if err != nil {
		t.Fatalf("查询数据失败: %v", err)
	}
	fmt.Println("查询到的所有用户:")
	for _, u := range users {
		fmt.Printf("ID: %d, 姓名: %s, 年龄: %d\n", u.ID, u.Name, u.Age)
	}

	// 更新数据
	user.Age = 26
	affected, err = engine.ID(user.ID).Update(user)
	if err != nil {
		t.Fatalf("更新数据失败: %v", err)
	}
	fmt.Printf("更新 %d 条记录\n", affected)
}
