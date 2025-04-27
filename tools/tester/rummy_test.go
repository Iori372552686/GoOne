package tester

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/random"
	g1_protocol "github.com/Iori372552686/game_protocol"
	pb "github.com/Iori372552686/game_protocol/protocol"
	"math"
	"sort"
	"strings"
	"testing"
)

// 复用德州卡牌
var (
	Wild     uint32 = uint32(random.Intn(11) + 1)
	WildFlag uint32 = 1 << 31

	color_name = map[pb.Color]string{
		pb.Color_COLOR_NONE:    "$",
		pb.Color_COLOR_DIAMOND: "♦",
		pb.Color_COLOR_CLUB:    "♣",
		pb.Color_COLOR_HEART:   "♥",
		pb.Color_COLOR_SPADE:   "♠",
	}
	color_value = map[string]uint32{
		"1": 0x00010000,
		"2": 0x00020000,
		"3": 0x00040000,
		"4": 0x00080000,
	}
	rank_name = map[pb.Rank]string{
		pb.Rank_RANK_1:  "A",
		pb.Rank_RANK_2:  "2",
		pb.Rank_RANK_3:  "3",
		pb.Rank_RANK_4:  "4",
		pb.Rank_RANK_5:  "5",
		pb.Rank_RANK_6:  "6",
		pb.Rank_RANK_7:  "7",
		pb.Rank_RANK_8:  "8",
		pb.Rank_RANK_9:  "9",
		pb.Rank_RANK_10: "10",
		pb.Rank_RANK_J:  "J",
		pb.Rank_RANK_Q:  "Q",
		pb.Rank_RANK_K:  "K",
		pb.Rank_RANK_A:  "A",
		pb.Rank(15):     "$",
	}
	rank_value = map[string]uint32{
		"2":  2,
		"3":  3,
		"4":  4,
		"5":  5,
		"6":  6,
		"7":  7,
		"8":  8,
		"9":  9,
		"10": 10,
		"J":  11,
		"Q":  12,
		"K":  13,
		"A":  14,
	}
)

type Card uint32
type CardList []uint32

func (d CardList) String() string {
	strs := []string{}
	for _, v := range d {
		strs = append(strs, Card(v).String())
	}
	return strings.Join(strs, ",")
}

func (d Card) Color() pb.Color {
	if d.IsWild() {
		return pb.Color_COLOR_NONE
	}
	return pb.Color(d >> 16)
}

func (d Card) Rank() pb.Rank {
	return pb.Rank(d & 0x0F)
}

func (d Card) Bit() uint32 {
	return 1 << (d.Rank() - 1)
}

func (t Card) Value() uint32 {
	return uint32(t)
}

func (c Card) String() string {
	return fmt.Sprintf("%s%s", rank_name[c.Rank()], color_name[c.Color()])
}

func (t Card) AddWild() uint32 {
	return uint32(t) | WildFlag
}

func (t Card) IsWild() bool {
	return (uint32(t) & WildFlag) != 0
}

// 牌堆初始化
// 德州扑克实现
type RummyPrivate struct {
	*g1_protocol.TexasGamePrivateData
}

// 发牌
func (p *RummyPrivate) Deal(count uint32, f func(uint32, uint32)) {
	for i := p.Cursor; i < p.Cursor+count; i++ {
		if f != nil {
			f(i, p.Cards[i])
		}
	}
	p.Cursor += count
}

// 洗牌
func (p *RummyPrivate) Init(times int) {
	p.Players = make(map[uint64]*g1_protocol.PlayerTexasGameCardData)
	// 初始化牌堆
	p.Cursor = 0
	p.Cards = p.Cards[:0]

	p.Cards = append(p.Cards, (1<<(16))|15) //joker
	for i := uint32(0); i <= 3; i++ {
		for j := uint32(2); j <= 14; j++ {
			if j == Wild {
				p.Cards = append(p.Cards, Card((1<<(16+i))|j).AddWild())
			} else {
				p.Cards = append(p.Cards, (1<<(16+i))|j)
			}
		}
	}
	// 洗牌
	for j := 0; j < times; j++ {
		for i := 0; i < len(p.Cards); i++ {
			pos := random.Intn(len(p.Cards))
			p.Cards[i], p.Cards[pos] = p.Cards[pos], p.Cards[i]
		}
	}
}

// 模拟手牌数据
func makeTestPrivateA() (private []uint32) {
	cardlist := []int{2, 3, 5, 6, 14}

	for _, item := range cardlist {
		card := (1 << (16 + 1)) | uint32(item)
		//if item == 6 {
		//	card = Card(card).AddWild()
		//}
		private = append(private, card)
	}

	return
}

func makeTestPrivateB() (private []uint32) {
	cardlist := []int{4, 5, 6, 7, 9}

	for _, item := range cardlist {
		card := (1 << (16 + 2)) | uint32(item)
		if item == 6 {
			card = Card(card).AddWild()
		}
		private = append(private, card)
	}

	return
}

func makeTestPrivateC() (private []uint32) {
	cardlist := []int{8, 9, 10, 11}

	for _, item := range cardlist {
		card := (1 << (16 + 3)) | uint32(item)
		if item == 6 {
			card = Card(card).AddWild()
		}
		private = append(private, card)
	}

	return
}

// 测试手牌
func TestPrivate(t *testing.T) {
	rummyPrivate := &RummyPrivate{
		TexasGamePrivateData: &g1_protocol.TexasGamePrivateData{},
	}

	//手牌是一个多维的数据结构 最多七组
	playerPrivate := make(map[int][]uint32, 7)

	rummyPrivate.Init(1)
	rummyPrivate.Deal(uint32(13), func(cursor uint32, card uint32) {
		//if is_set := playerPrivate[int(Card(card).Color())]; is_set == nil {
		//	playerPrivate[int(Card(card).Color())] = make([]uint32, 0, 14)
		//}
		//playerPrivate[int(Card(card).Color())] = append(playerPrivate[int(Card(card).Color())], card)
		////发牌
		//t.Logf("card cursor: %d , color: %d ,rank :%d", cursor, Card(card).Color(), int(Card(card).Rank()))
	})

	//aceA := (1 << (16 + 1)) | 14
	//aceB := (1 << (16 + 2)) | 14
	//playerPrivate[int(Card(aceA).Color())] = append(playerPrivate[int(Card(aceA).Color())], uint32(aceA))
	//playerPrivate[int(Card(aceB).Color())] = append(playerPrivate[int(Card(aceB).Color())], uint32(aceB))
	playerPrivate[1] = makeTestPrivateA()
	playerPrivate[2] = makeTestPrivateB()
	playerPrivate[3] = makeTestPrivateC()

	//排序次数
	for _, cards := range playerPrivate {
		//默认升序
		sort.Slice(cards, func(i, j int) bool {
			return cards[i] < cards[j]
		})
		t.Logf("cards: %s", CardList(cards).String())
		//cardType, score := GetCardType(cards)
		//t.Logf("cardType: %s , score : %d", cardType.String(), score)
	}
	ready, score := GetCardValue(playerPrivate)
	t.Logf("cardType: %t , score : %d", ready, score)
	// private := &Private{TexasGamePrivateData: data.Table.PrivateData}
}

// 比牌
// 获取手牌积分 首顺0分 满足首顺情况下赖顺0分 满足前二后顺子刻子0分
func GetCardValue(playerPrivate map[int][]uint32) (ready bool, score uint32) {
	isFirst := false
	secondCount := uint32(0)
	isSecond := false
	otherCount := uint32(0)

	for _, set := range playerPrivate {
		cardType, scoreItem := GetCardType(set)
		switch cardType {
		case pb.CardType_HIGH_CARD:
			otherCount += scoreItem
		case pb.CardType_STRAIGHT_FLUSH:
			if isFirst {
				isSecond = true
			}
			isFirst = true
		case pb.CardType_STRAIGHT:
			isSecond = true
			secondCount += scoreItem
		case pb.CardType_THREE_OF_A_KIND:
			secondCount += scoreItem
		}
	}

	score += otherCount
	if !(isFirst && isSecond) {
		score += secondCount
	} else if score == 0 {
		ready = true
	}
	return
}

// rummy比牌算法
func GetCardType(vals []uint32) (cardType pb.CardType, score uint32) {
	lenC := len(vals)
	cardType = pb.CardType_HIGH_CARD

	isWild := false
	numWild := 0
	bit := uint32(0)
	minCardRank := math.MaxInt32 //除百搭以外最小牌

	hasAce := false

	color := pb.Color(0)
	for _, card := range vals { // todo double A 10分
		if Card(card).IsWild() { //joker todo wild
			isWild = true
			numWild++
		} else {
			if Card(card).Rank() == g1_protocol.Rank_RANK_A {
				bit |= 1
				hasAce = true
			}

			bit |= Card(card).Bit()
			color |= Card(card).Color()

			scoreItem := uint32(Card(card).Rank())
			if scoreItem >= 10 {
				score += 10
			} else {
				score += scoreItem
			}

			if minCardRank > int(scoreItem-1) {
				minCardRank = int(scoreItem - 1)
			}
		}
	}

	if lenC < 3 {
		return
	}

	//如果带A 判断两次顺 结果取 1一次 || 14一次
	if color == pb.Color_COLOR_DIAMOND || color == pb.Color_COLOR_CLUB || color == pb.Color_COLOR_HEART || color == pb.Color_COLOR_SPADE {
		//同花 判断首顺 赖顺
		if isWild {
			if lenC-numWild == 1 {
				// second
				cardType = pb.CardType_STRAIGHT
			} else {
				fmt.Printf("源数据高位A: %032b ,比对: %032b \n", bit>>minCardRank, (1<<lenC)-1)
				gap := countZerosBetweenOnes(bit >> minCardRank)
				if hasAce {
					gap = min(gap, countZerosBetweenOnes(bit&^(1<<13)))
				}

				if gap <= numWild {
					cardType = pb.CardType_STRAIGHT
				}
			}
		} else {
			//判断非赖顺
			// fmt.Printf("源数据高位A: %032b ,比对: %032b \n", bit>>minCardRank, (1<<lenC)-1)
			// fmt.Printf("源数据低位A: %032b ,比对: %032b \n", bit&^(1<<13), (1<<lenC)-1)
			if (bit>>minCardRank) == (1<<lenC)-1 || (hasAce && bit&^(1<<13) == (1<<lenC)-1) {
				// must first
				cardType = pb.CardType_STRAIGHT_FLUSH
			}
		}
	} else {
		if lenC <= 4 && minCardRank != math.MaxInt32 && bit == 1<<minCardRank {
			cardType = pb.CardType_THREE_OF_A_KIND
		} else if isWild && numWild == 5 {
			cardType = pb.CardType_STRAIGHT
		}
	}
	return

}

func countZerosBetweenOnes(n uint32) (result int) {
	count := 0
	inSequence := false // 是否已经遇到第一个 1

	for i := 0; i < 14; i++ {
		if (n & (1 << i)) != 0 { // 当前位是 1
			if inSequence && count > 0 {
				result += count // 记录 0 的数量
			}
			inSequence = true
			count = 0 // 重置计数
		} else if inSequence { // 当前位是 0，且之前遇到了 1
			count++
		}
	}

	return
}

// 打牌 todo
