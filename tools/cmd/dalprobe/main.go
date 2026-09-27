// dalprobe DAL 三层持久化联调探针：直接操作 Redis(L2)/MySQL(L3)，
// 供 persist 组件两阶段（write→wipe→verify）编排使用。
//
// 用法：
//
//	dalprobe -op wipe-l2   -uid 100001 -redis.ip 127.0.0.1 -redis.port 6379 [-redis.pass xx]
//	dalprobe -op check-l2  -uid 100001 ...            # 输出 EXISTS/TTL/FIELD 数
//	dalprobe -op check-l3  -uid 100001 -mysql.dsn "user:pass@tcp(ip:3306)/db"
//
// 退出码：0=成功（check 类含断言通过），1=失败。
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	_ "github.com/go-sql-driver/mysql"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	op := flag.String("op", "", "wipe-l2 | check-l2 | check-l3")
	uid := flag.Uint64("uid", 0, "玩家 uid")
	redisIP := flag.String("redis.ip", "127.0.0.1", "Redis 地址")
	redisPort := flag.Int("redis.port", 6379, "Redis 端口")
	redisPass := flag.String("redis.pass", "", "Redis 密码")
	mysqlDSN := flag.String("mysql.dsn", "", "MySQL DSN（user:pass@tcp(ip:port)/db）")
	flag.Parse()

	if *op == "" || *uid == 0 {
		fmt.Fprintln(os.Stderr, "需要 -op 与 -uid")
		os.Exit(1)
	}
	key := fmt.Sprintf("%s:%d", g1_protocol.DBType_DB_TYPE_ROLE.String(), *uid)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	switch *op {
	case "wipe-l2", "check-l2":
		err = redisOp(ctx, *op, key, *redisIP, *redisPort, *redisPass)
	case "check-l3":
		if *mysqlDSN == "" {
			fmt.Fprintln(os.Stderr, "check-l3 需要 -mysql.dsn")
			os.Exit(1)
		}
		err = mysqlOp(ctx, *uid, *mysqlDSN)
	default:
		err = fmt.Errorf("未知 op %q", *op)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[dalprobe] %s uid=%d: %v\n", *op, *uid, err)
		os.Exit(1)
	}
}

func redisClient(ip string, port int, pass string) *goredis.Client {
	return goredis.NewClient(&goredis.Options{Addr: fmt.Sprintf("%s:%d", ip, port), Password: pass})
}

func redisOp(ctx context.Context, op, key, ip string, port int, pass string) error {
	cli := redisClient(ip, port, pass)
	defer cli.Close()

	switch op {
	case "wipe-l2":
		// 模拟 L2 丢失（TTL 过期 / 清库）。DEL 前打印现场，便于报告取证。
		exists, err := cli.Exists(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("exists: %w", err)
		}
		ttl, _ := cli.TTL(ctx, key).Result()
		fields, _ := cli.HLen(ctx, key).Result()
		fmt.Printf("[dalprobe] before wipe: exists=%d ttl=%s fields=%d\n", exists, ttl, fields)
		if exists == 0 {
			return fmt.Errorf("L2 key 不存在，wipe 无意义（先跑 write 阶段）")
		}
		if err := cli.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("del: %w", err)
		}
		fmt.Printf("[dalprobe] wiped L2 key=%s（后续登录必须从 L3 恢复）\n", key)
		return nil
	case "check-l2":
		exists, err := cli.Exists(ctx, key).Result()
		if err != nil {
			return err
		}
		ttl, err := cli.TTL(ctx, key).Result()
		if err != nil {
			return err
		}
		fields, err := cli.HLen(ctx, key).Result()
		if err != nil {
			return err
		}
		fmt.Printf("[dalprobe] L2 key=%s exists=%d ttl=%s fields=%d\n", key, exists, ttl, fields)
		if exists == 1 {
			// TTL 配置为 30 天时，活跃写入后的 TTL 应 > 0 且 <= 30d。
			if ttl <= 0 {
				fmt.Println("[dalprobe] WARN: TTL=-1（永不过期）——role_cache_ttl_days 未生效")
			} else if ttl > 30*24*time.Hour {
				return fmt.Errorf("TTL=%s 超过 30 天上限", ttl)
			}
			if fields < 10 {
				return fmt.Errorf("fields=%d 过少（全量段缺失？）", fields)
			}
		}
		return nil
	}
	return nil
}

func mysqlOp(ctx context.Context, uid uint64, dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	var dataLen, updateTime int64
	err = db.QueryRowContext(ctx,
		"SELECT LENGTH(data), update_time FROM role_data WHERE uid = ?", uid).
		Scan(&dataLen, &updateTime)
	if err == sql.ErrNoRows {
		return fmt.Errorf("L3 无 role_data 行（快照未落库：MaybeFlushL3 未生效或投递失败）")
	}
	if err != nil {
		return err
	}
	fmt.Printf("[dalprobe] L3 role_data uid=%d data=%dB update_time=%d (%s)\n",
		uid, dataLen, updateTime, time.UnixMilli(updateTime).Format(time.RFC3339))
	if dataLen < 100 {
		return fmt.Errorf("快照过小 data=%dB（不完整的 RoleInfo？）", dataLen)
	}
	return nil
}
