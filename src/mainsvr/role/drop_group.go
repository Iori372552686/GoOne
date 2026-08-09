/// 掉落系统：双层模型（掉落组 → 掉落包 → 物品）
// 移植自 seed-server component/base/drop.go，本土化为 *Role 方法 + 纯函数。
// 核心算法（resolveGroup/selectOnce/pickKeyAsDraws）保持原语义，去掉 Actor/Component 框架。

package role

import (
	"math/rand"
	"sort"

	"github.com/Iori372552686/GoOne/module/gamedata/repository/drop_group_config"
	"github.com/Iori372552686/GoOne/module/gamedata/repository/drop_item_config"
	pb "github.com/Iori372552686/g1_common/protocol"
)

// 掉落组 DropWay（选择方式，只决定"怎么选"）
const (
	dropWayProb   int32 = 1 // 概率随机：dropRateMap 每个 key 按 value(万分比) 独立判定
	dropWayFixed  int32 = 2 // 固定全掉：dropRateMap 全部 key 都产出
	dropWayWeight int32 = 3 // 权重随机：按 value 权重选 1 个 key
)

// 掉落组 DropType（产出控制）
const (
	dropTypeMultiply int32 = 0 // 结果×N：一次选择后整体乘倍数
	dropTypeSelectN  int32 = 1 // 选N次：独立执行 N 次
)

// 掉落组 Type（组类型）
const (
	dropGroupTypePack  int32 = 0 // 叶子掉落包：dropRateMap 的 key 直接当 DropId 产出
	dropGroupTypeCombo int32 = 1 // 组合包：遍历该组全部启用行，key 可指向子组(递归)
)

// 掉落包 item 级 DropWay
const (
	dropItemWayAll      int32 = 1 // 全产出
	dropItemWayRandom1  int32 = 2 // 随机选1
	dropItemWayWeight1  int32 = 3 // 权重选1
	dropItemWayIndep    int32 = 4 // 独立概率(每项按自身 probability 万分比判定)
)

const (
	maxDropNestDepth    int32 = 6   // 子掉落组递归最大深度
	maxDropExecuteCount int32 = 100 // DropExecute 最大执行次数
)

// packDraw 一次掉落包命中：dropID=掉落包id，mult=产出倍数。
type packDraw struct {
	dropID int32
	mult   int32
}

// DropExecute 双层掉落主入口：解析 dropGroupID → 多次执行 → 聚合 → 走 ItemAdd 落账。
// 返回聚合后的产出列表（已落账到背包）。
// dropGroupID 通常是 DropGroupConfig.Groupid；若该 id 不在掉落组表，降级查 dropItem 表直接产出。
func (r *Role) DropExecute(dropGroupID int32, count int32, reason *Reason) *[]*pb.PbItem {
	if count <= 0 {
		count = 1
	}
	if count > maxDropExecuteCount {
		count = maxDropExecuteCount
	}

	// 幂等/快照：单次请求内多次执行，配置可能被热更；这里每次实时读，不做快照（数据量小）。
	aggregated := make(map[int32]int64)
	for i := int32(0); i < count; i++ {
		items := r.executeDropOnce(dropGroupID, reason)
		for _, it := range items {
			aggregated[it.Id] += it.Count
		}
	}

	out := make([]*pb.PbItem, 0, len(aggregated))
	for id, cnt := range aggregated {
		if cnt <= 0 {
			continue
		}
		// 产出落账到背包/货币
		r.ItemAdd(id, cnt, reason)
		out = append(out, &pb.PbItem{Id: id, Count: cnt})
	}
	return &out
}

// executeDropOnce 执行一次掉落解析：返回本次命中的物品列表（未落账）。
func (r *Role) executeDropOnce(dropGroupID int32, reason *Reason) []*pb.PbItem {
	draws := resolveGroup(dropGroupID, 0, map[int32]bool{dropGroupID: true})
	if len(draws) == 0 {
		// 降级：dropGroupID 不在掉落组表 → 当作 DropId 直接查 dropItem
		return expandDropId(dropGroupID, 1)
	}

	var items []*pb.PbItem
	for _, d := range draws {
		if d.mult <= 0 {
			continue
		}
		items = append(items, expandDropId(d.dropID, d.mult)...)
	}
	return items
}

// expandDropId 按 DropId 查 dropItem 表，按 item 级 DropWay 展开产出。
func expandDropId(dropID int32, mult int32) []*pb.PbItem {
	entries := drop_item_config.GetByDropId(dropID)
	if len(entries) == 0 {
		return nil
	}
	selected := randomItemEntries(entries)
	out := make([]*pb.PbItem, 0, len(selected))
	for _, e := range selected {
		cnt := int64(e.Count) * int64(mult)
		if cnt <= 0 {
			continue
		}
		out = append(out, &pb.PbItem{Id: e.ItemId, Count: cnt})
	}
	return out
}

// resolveGroup 解析一个 groupid，返回完全展开后的掉落包列表（packDraw）。
// depth 限制递归深度(maxDropNestDepth)，visited 防止 groupid 循环引用。
// 纯函数（不依赖 Role），便于单测。
func resolveGroup(groupID int32, depth int32, visited map[int32]bool) []packDraw {
	rows := drop_group_config.GetByGroupid(groupID)
	if len(rows) == 0 {
		return nil
	}
	root := rootRow(rows)
	if root == nil || root.IsBan != 0 {
		return nil
	}

	enabled := enabledRowsSorted(rows)
	if len(enabled) == 0 {
		return nil
	}
	isLeaf := root.Type != dropGroupTypeCombo
	var draws []packDraw
	for _, row := range enabled {
		draws = append(draws, resolveRow(row, depth, visited, isLeaf)...)
	}
	return draws
}

// rootRow 取该 groupid 的根行（subid 最小，约定同组同 type）。
func rootRow(rows []*pb.DropGroupConfig) *pb.DropGroupConfig {
	var root *pb.DropGroupConfig
	for _, r := range rows {
		if r == nil {
			continue
		}
		if root == nil || r.Subid < root.Subid {
			root = r
		}
	}
	return root
}

// enabledRowsSorted 取启用行(IsBan==0)并按 subid 升序，保证遍历顺序稳定。
func enabledRowsSorted(rows []*pb.DropGroupConfig) []*pb.DropGroupConfig {
	out := make([]*pb.DropGroupConfig, 0, len(rows))
	for _, r := range rows {
		if r != nil && r.IsBan == 0 {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Subid < out[j].Subid })
	return out
}

// resolveRow 解析单个掉落组行，返回命中的掉落包列表（含倍数）。
func resolveRow(row *pb.DropGroupConfig, depth int32, visited map[int32]bool, isLeaf bool) []packDraw {
	if row == nil {
		return nil
	}
	n := row.DropCount
	if n <= 0 {
		n = 1
	}
	if row.DropType == dropTypeMultiply {
		base := selectOnce(row, depth, visited, isLeaf)
		if n > 1 {
			for i := range base {
				base[i].mult *= n
			}
		}
		return base
	}
	// dropType=1：独立执行 n 次选择并聚合
	out := make([]packDraw, 0, n)
	for i := int32(0); i < n; i++ {
		out = append(out, selectOnce(row, depth, visited, isLeaf)...)
	}
	return out
}

// selectOnce 执行本行的一次"选择动作"，返回该次选择命中的掉落包（mult=1）。
func selectOnce(row *pb.DropGroupConfig, depth int32, visited map[int32]bool, isLeaf bool) []packDraw {
	pick := func(k int32) []packDraw {
		if isLeaf {
			return []packDraw{{dropID: k, mult: 1}}
		}
		return pickKeyAsDraws(k, depth, visited)
	}
	switch row.DropWay {
	case dropWayFixed:
		keys := sortedKeys(row.DropRateMap)
		out := make([]packDraw, 0, len(keys))
		for _, k := range keys {
			out = append(out, pick(k)...)
		}
		return out
	case dropWayProb:
		keys := sortedKeys(row.DropRateMap)
		out := make([]packDraw, 0, len(keys))
		for _, k := range keys {
			if probabilityHit(row.DropRateMap[k]) {
				out = append(out, pick(k)...)
			}
		}
		return out
	case dropWayWeight:
		k, ok := weightedPickKey(row.DropRateMap)
		if !ok {
			return nil
		}
		return pick(k)
	default:
		return nil
	}
}

// pickKeyAsDraws 把选中的 key 转成 draws（仅组合包 type=1 调用）：
//   - key 在 DropGroupConfig 表 → 递归 resolveGroup 展开
//   - key 不在表 → 视为纯 DropId，直接产出
func pickKeyAsDraws(k int32, depth int32, visited map[int32]bool) []packDraw {
	if len(drop_group_config.GetByGroupid(k)) == 0 {
		return []packDraw{{dropID: k, mult: 1}}
	}
	if depth >= maxDropNestDepth {
		return nil
	}
	if visited[k] {
		return nil
	}
	nv := make(map[int32]bool, len(visited)+1)
	for g := range visited {
		nv[g] = true
	}
	nv[k] = true
	return resolveGroup(k, depth+1, nv)
}

// ---- 掉落包 -> 物品（item 级 dropWay）----

func randomItemEntries(entries []*pb.DropItemConfig) []*pb.DropItemConfig {
	if len(entries) == 0 {
		return nil
	}
	dropWay := entries[0].DropWay
	switch dropWay {
	case dropItemWayAll:
		return entries
	case dropItemWayRandom1:
		return []*pb.DropItemConfig{entries[rand.Intn(len(entries))]}
	case dropItemWayWeight1:
		selected := weightedRandomItemEntry(entries)
		if selected == nil {
			return nil
		}
		return []*pb.DropItemConfig{selected}
	case dropItemWayIndep:
		out := make([]*pb.DropItemConfig, 0, len(entries))
		for _, e := range entries {
			if probabilityHit(e.Probability) {
				out = append(out, e)
			}
		}
		return out
	default:
		return nil
	}
}

func weightedRandomItemEntry(entries []*pb.DropItemConfig) *pb.DropItemConfig {
	total := 0
	for _, e := range entries {
		total += int(e.Probability)
	}
	if total <= 0 {
		return nil
	}
	rr := rand.Intn(total)
	for _, e := range entries {
		rr -= int(e.Probability)
		if rr < 0 {
			return e
		}
	}
	return entries[len(entries)-1]
}

// ---- 随机辅助 ----

// probabilityHit 按万分比判定；rate<=0 永不命中，rate>=10000 必定命中。
func probabilityHit(rate int32) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 10000 {
		return true
	}
	return rand.Intn(10000) < int(rate)
}

func sortedKeys(m map[int32]int32) []int32 {
	keys := make([]int32, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func weightedPickKey(m map[int32]int32) (int32, bool) {
	keys := sortedKeys(m)
	total := 0
	for _, k := range keys {
		if w := int(m[k]); w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return 0, false
	}
	rr := rand.Intn(total)
	for _, k := range keys {
		w := int(m[k])
		if w <= 0 {
			continue
		}
		rr -= w
		if rr < 0 {
			return k, true
		}
	}
	return keys[len(keys)-1], true
}
