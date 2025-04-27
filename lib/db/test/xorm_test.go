package test

import (
	"fmt"
	"log"
	"testing"

	g1_protocol "github.com/Iori372552686/game_protocol"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

var engine *xorm.Engine

func TestMain(m *testing.M) {
	// 创建xorm数据库连接
	cnn, err := xorm.NewEngine("mysql", "root:123456@tcp(127.0.0.1:3306)/testdb?charset=utf8")
	if err != nil {
		panic(err)
	}

	if err := cnn.Sync2(new(g1_protocol.TexasData)); err != nil {
		panic(err)
	}

	engine = cnn
	m.Run()
}

func TestXorm(t *testing.T) {
	item := &g1_protocol.TexasData{
		RoomId:   1,
		CurState: g1_protocol.GameState_STATE_START,
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
	// 同步结构体到数据库表结构
	if err := engine.Sync2(new(User)); err != nil {
		log.Fatalf("同步表结构失败: %v", err)
	}

	// 插入数据
	user := &User{Name: "Alice", Age: 25}
	affected, err := engine.Insert(user)
	if err != nil {
		log.Fatalf("插入数据失败: %v", err)
	}
	fmt.Printf("插入 %d 条记录\n", affected)

	// 查询数据
	var users []User
	err = engine.Find(&users)
	if err != nil {
		log.Fatalf("查询数据失败: %v", err)
	}
	fmt.Println("查询到的所有用户:")
	for _, u := range users {
		fmt.Printf("ID: %d, 姓名: %s, 年龄: %d\n", u.ID, u.Name, u.Age)
	}

	// 更新数据
	user.Age = 26
	affected, err = engine.ID(user.ID).Update(user)
	if err != nil {
		log.Fatalf("更新数据失败: %v", err)
	}
	fmt.Printf("更新 %d 条记录\n", affected)

	/*
		// 删除数据
		affected, err = engine.ID(user.ID).Delete(new(User))
		if err != nil {
			log.Fatalf("删除数据失败: %v", err)
		}
		fmt.Printf("删除 %d 条记录\n", affected)
	*/

}
