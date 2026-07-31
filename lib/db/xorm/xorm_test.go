package orm

import (
	"fmt"
	"os"
	"testing"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
)

// testDSN 返回本地 MySQL 测试 DSN。可通过 GOONE_MYSQL_DSN 覆盖，默认指向本地 testdb。
// 仅用于集成测试，且仅在 GOONE_INTEGRATION=1 时才会被访问。
func testDSN() string {
	if dsn := os.Getenv("GOONE_MYSQL_DSN"); dsn != "" {
		return dsn
	}
	return "root:123456@tcp(127.0.0.1:3306)/testdb?charset=utf8"
}

// newTestEngine 建立 xorm engine 并同步测试表。调用方需先通过 itest.Require 门控。
func newTestEngine(t *testing.T, tables ...interface{}) *xorm.Engine {
	t.Helper()
	cnn, err := xorm.NewEngine("mysql", testDSN())
	if err != nil {
		t.Fatalf("connect mysql: %v", err)
	}
	if err := cnn.Ping(); err != nil {
		_ = cnn.Close()
		t.Fatalf("ping mysql: %v", err)
	}
	for _, tbl := range tables {
		if err := cnn.Sync2(tbl); err != nil {
			_ = cnn.Close()
			t.Fatalf("sync table: %v", err)
		}
	}
	return cnn
}

// 删除原 TestMain 的 os.Exit(0) 跳过整个包的做法（它会掩盖真实失败，
// 且让 CI 无法区分"跳过"与"通过"）。改为每个集成测试函数独立用 itest.Require 门控：
// 未开启集成模式或 mysql 不可达时 t.Skip，而非以 0 退出码退出进程。

func TestXorm(t *testing.T) {
	// 集成测试统一门控。
	itest.Require(t, "127.0.0.1:3306")
	engine := newTestEngine(t, new(g1_protocol.MysqlTexasRoomInfo))
	defer engine.Close()

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
	// 集成测试统一门控。
	itest.Require(t, "127.0.0.1:3306")
	engine := newTestEngine(t, new(User))
	defer engine.Close()

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
