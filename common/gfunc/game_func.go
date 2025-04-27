package gfunc

import (
	"encoding/binary"
	"fmt"
	"github.com/Iori372552686/GoOne/common/define"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	pb "github.com/Iori372552686/game_protocol/protocol"
	"strconv"
)

// CRC32（32 位）冲突概率 ≈ 50% 当生成 7.7 万个 ID 时（生日问题）
// xxHash（64 位）冲突概率 ≈ 50% 当生成 50 亿个 ID 时
// 实际应根据业务量选择：
// 中小规模（<1M ID/天）：CRC32 足够
// 超大规模：推荐 xxHash 或 SHA1 截断
func GenerateRoomId(iDGen uint64) uint64 {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, iDGen)
	//hash := uint64(crc32.ChecksumIEEE(buf))  // 生成 32 位哈希,crc32 算法
	strHash := fmt.Sprintf("%d", xxhash.Sum64(buf))
	roomID, _ := strconv.ParseUint(strHash[:define.RoomIdLen], 10, 64)
	return roomID
}

// GetTexasRoomListIndex 获取德州房间列表索引
func GetTexasRoomListIndex(zone uint32, gameId pb.GameTypeId, coin pb.CoinType) uint64 {
	logger.Infof("GetTexasRoomListIndex zone:%v gameId:%v coin:%v", zone, gameId, coin)
	return uint64(gameId)*10000 + uint64(zone)*10 + uint64(coin)
}
