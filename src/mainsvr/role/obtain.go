/// 恭喜获得系统：统一聚合奖励展示数据，不承担资产落账的权威存储职责。
// 移植自 seed-server component/base/obtain.go，本土化为 *Role 方法。
// 资产状态仍以背包/货币同步协议（CMD_SC_SYNC_USER_DATA_V2）为准；
// 本系统只做展示聚合推送（CMD_SC_OBTAIN_NOTICE），与资产存储解耦。

package role

import (
	"sort"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/service/router"
	itemconf "github.com/Iori372552686/GoOne/module/gamedata/repository/item"
	"github.com/Iori372552686/GoOne/module/gamedata/repository/obtain"
	pb "github.com/Iori372552686/g1_common/protocol"
)

const (
	obtainDefaultMaxShowCount   = 20
	obtainDefaultRarePopup      = 4
	obtainMaxConfiguredShow     = 100
	obtainRewardTypeItem        = 1
	obtainRewardTypeCurrency    = 2
)

const (
	obtainMergeModeNone     int32 = 0 // 不合并
	obtainMergeModeItemID   int32 = 1 // 按 itemId 合并
	obtainMergeModeQuality  int32 = 2 // 按品质保留
)

// obtainState 每个玩家的获得展示运行时状态（source 冷却）。
type obtainState struct {
	sourceCooldown map[string]int64 // source -> 上次推送的 unix nano
}

var obtainStates sync.Map // uid(uint64) -> *obtainState

func getObtainState(uid uint64) *obtainState {
	v, _ := obtainStates.LoadOrStore(uid, &obtainState{sourceCooldown: make(map[string]int64)})
	return v.(*obtainState)
}

// ObtainNotifyParam 获得展示通知参数。
type ObtainNotifyParam struct {
	RequestID   string
	Source      string // 来源标识(对应 ObtainPolicyConfig.Source)
	SourceRefID int32
	DisplayMode pb.ObtainDisplayMode // 覆盖策略的展示模式；0 表示用策略默认
	Items       []*pb.PbItem         // 本次产出的道具/货币列表
}

// ObtainNotify 主入口：策略路由 → 冷却判定 → 合并/排序/截断 → 推送。
// 返回 0 成功，非 0 错误码。
func (r *Role) ObtainNotify(param *ObtainNotifyParam) pb.ErrorCode {
	if param == nil || param.Source == "" {
		return pb.ErrorCode_ERR_ARGV
	}

	policy := obtainPolicyForSource(param.Source)
	if param.DisplayMode != pb.ObtainDisplayMode_OBTAIN_DISPLAY_NONE {
		policy.displayMode = param.DisplayMode
	}

	// source 冷却：冷却期内不推送（防刷屏）
	st := getObtainState(r.Uid())
	if isSourceCoolingDown(st, param.Source, policy) {
		return pb.ErrorCode_ERR_OK
	}

	items, promoted := buildObtainRewardItems(param, policy)
	if promoted && policy.displayMode == pb.ObtainDisplayMode_OBTAIN_DISPLAY_TIPS {
		policy.displayMode = pb.ObtainDisplayMode_OBTAIN_DISPLAY_POPUP
	}
	if policy.displayMode == pb.ObtainDisplayMode_OBTAIN_DISPLAY_NONE || len(items) == 0 {
		return pb.ErrorCode_ERR_OK
	}

	totalCount := int32(len(items))
	hasMore := false
	if policy.maxShowCount > 0 && len(items) > policy.maxShowCount {
		items = items[:policy.maxShowCount]
		hasMore = true
	}

	connsvrBusID := r.PbRole.ConnSvrInfo.BusId
	if connsvrBusID == 0 {
		return pb.ErrorCode_ERR_NOT_EXIST_PLAYER
	}
	msg := &pb.S2CObtainNotice{
		RequestId:       param.RequestID,
		Source:          param.Source,
		SourceRefId:     param.SourceRefID,
		DisplayMode:     policy.displayMode,
		Items:           items,
		HasMore:         hasMore,
		TotalItemCount:  totalCount,
		ServerTime:      time.Now().Unix(),
	}
	if err := router.SendPbMsgByBusIdSimple(connsvrBusID, r.Uid(), pb.CMD_SC_OBTAIN_NOTICE, msg); err != nil {
		r.Errorf("OBTAIN|push failed {source:%s, err:%v}", param.Source, err)
		return pb.ErrorCode_ERR_INTERNAL
	}
	rememberSourcePush(st, param.Source)
	return pb.ErrorCode_ERR_OK
}

// ObtainNotifyItems 便捷方法：给一组 PbItem 触发展示。
func (r *Role) ObtainNotifyItems(source string, items []*pb.PbItem) pb.ErrorCode {
	if len(items) == 0 {
		return pb.ErrorCode_ERR_OK
	}
	return r.ObtainNotify(&ObtainNotifyParam{
		RequestID: source,
		Source:    source,
		Items:     items,
	})
}

// ---- 展示策略 ----

type obtainPolicy struct {
	displayMode      pb.ObtainDisplayMode
	mergeMode        int32
	minQuality       int32
	includeCurrency  bool
	maxShowCount     int
	rarePopupQuality int32
	cooldown         time.Duration
}

func obtainPolicyForSource(source string) obtainPolicy {
	policy := defaultObtainPolicyForSource(source)
	cfg := obtain.GetObtainPolicyBySource(source)
	if cfg == nil || cfg.IsEnable == 0 {
		return policy
	}
	if validObtainDisplayMode(cfg.DisplayMode) {
		policy.displayMode = pb.ObtainDisplayMode(cfg.DisplayMode)
	}
	if validObtainMergeMode(cfg.MergeMode) {
		policy.mergeMode = cfg.MergeMode
	}
	if cfg.MinQuality >= 0 {
		policy.minQuality = cfg.MinQuality
	}
	if cfg.IncludeCurrency == 0 || cfg.IncludeCurrency == 1 {
		policy.includeCurrency = cfg.IncludeCurrency == 1
	}
	if cfg.MaxShowCount > 0 {
		policy.maxShowCount = int(cfg.MaxShowCount)
		if policy.maxShowCount > obtainMaxConfiguredShow {
			policy.maxShowCount = obtainMaxConfiguredShow
		}
	}
	if cfg.RarePopupQuality > 0 {
		policy.rarePopupQuality = cfg.RarePopupQuality
	}
	if cfg.CooldownMs > 0 {
		policy.cooldown = time.Duration(cfg.CooldownMs) * time.Millisecond
	}
	return policy
}

func defaultObtainPolicyForSource(source string) obtainPolicy {
	policy := obtainPolicy{
		displayMode:      pb.ObtainDisplayMode_OBTAIN_DISPLAY_TIPS,
		mergeMode:        obtainMergeModeItemID,
		includeCurrency:  true,
		maxShowCount:     obtainDefaultMaxShowCount,
		rarePopupQuality: obtainDefaultRarePopup,
	}
	switch source {
	case "mail", "milestone_reward":
		policy.displayMode = pb.ObtainDisplayMode_OBTAIN_DISPLAY_POPUP
	case "match_settle":
		policy.displayMode = pb.ObtainDisplayMode_OBTAIN_DISPLAY_SETTLE
	case "silent":
		policy.displayMode = pb.ObtainDisplayMode_OBTAIN_DISPLAY_NONE
	}
	return policy
}

func validObtainDisplayMode(mode int32) bool {
	return mode >= int32(pb.ObtainDisplayMode_OBTAIN_DISPLAY_NONE) &&
		mode <= int32(pb.ObtainDisplayMode_OBTAIN_DISPLAY_SETTLE)
}

func validObtainMergeMode(mode int32) bool {
	return mode >= obtainMergeModeNone && mode <= obtainMergeModeQuality
}

// ---- 合并 / 排序 / 填充 ----

type obtainMergeKey struct {
	rewardType int32
	itemID     int32
	seq        int
}

func buildObtainRewardItems(param *ObtainNotifyParam, policy obtainPolicy) ([]*pb.ObtainRewardItem, bool) {
	merged := make(map[obtainMergeKey]*pb.ObtainRewardItem)
	rarePromoted := false
	seq := 0

	add := func(it *pb.PbItem) {
		if it == nil || it.Id <= 0 || it.Count <= 0 {
			return
		}
		rewardType := int32(obtainRewardTypeItem)
		if isBasicInfoItem(it.Id) {
			if !policy.includeCurrency {
				return
			}
			rewardType = obtainRewardTypeCurrency
		}
		item := &pb.ObtainRewardItem{
			RewardType: rewardType,
			ItemId:     it.Id,
			Count:      it.Count,
		}
		fillObtainDisplayInfo(item)
		if rewardType != obtainRewardTypeCurrency && policy.minQuality > 0 && item.Quality < policy.minQuality {
			return
		}
		if item.Quality >= policy.rarePopupQuality {
			item.Rare = true
			rarePromoted = true
		}
		key := obtainMergeKey{rewardType: rewardType, itemID: it.Id}
		if policy.mergeMode == obtainMergeModeNone {
			key.seq = seq
			seq++
		}
		if existing, ok := merged[key]; ok {
			existing.Count += it.Count
			return
		}
		merged[key] = item
	}

	for _, it := range param.Items {
		add(it)
	}

	items := make([]*pb.ObtainRewardItem, 0, len(merged))
	for _, item := range merged {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Rare != items[j].Rare {
			return items[i].Rare
		}
		if items[i].Quality != items[j].Quality {
			return items[i].Quality > items[j].Quality
		}
		if items[i].RewardType != items[j].RewardType {
			return items[i].RewardType < items[j].RewardType
		}
		return items[i].ItemId < items[j].ItemId
	})
	return items, rarePromoted
}

// fillObtainDisplayInfo 从 ItemConfig 填充 Name/Icon/Quality。
func fillObtainDisplayInfo(item *pb.ObtainRewardItem) {
	cfg := itemconf.GetItemByItemId(item.ItemId)
	if cfg == nil {
		return
	}
	item.Name = cfg.Name
	item.Icon = cfg.Icon
	item.Quality = cfg.Quality
}

// ---- 冷却 ----

func isSourceCoolingDown(st *obtainState, source string, policy obtainPolicy) bool {
	if source == "" || policy.cooldown <= 0 {
		return false
	}
	last, ok := st.sourceCooldown[source]
	if !ok {
		return false
	}
	return time.Since(time.Unix(0, last)) < policy.cooldown
}

func rememberSourcePush(st *obtainState, source string) {
	if source == "" {
		return
	}
	st.sourceCooldown[source] = time.Now().UnixNano()
}
