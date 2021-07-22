package role

// 数据变化原因
type Reason struct {
	Reason    int32
	SubReason int32
}

// 数据变化原因枚举
const (
	REASON_INIT                      = 1
	REASON_LOGIN                     = 1000
	REASON_GM                        = 1001
	REASON_CONSUME                   = 1002
	REASON_MALL_BUY                  = 1003
	REASON_INSTANCE                  = 1005
	REASON_INSTANCE_FAIL             = 1006
	REASON_CHAPTOR_REWARD            = 1007
	REASON_RES_INSTANCE              = 1008
	REASON_INSTANCE_QUICK            = 1009
	REASON_RES_INSTANCE_QUICK        = 1010
	REASON_RENAME                    = 1011
	REASON_MAIL_ATTACH               = 1012
	REASON_RECRUIT                   = 1013
	REASON_HERO_UPGRADE_LEVEL        = 1014
	REASON_HERO_UPGRADE_STAR         = 1015
	REASON_HERO_UPGRADE_SKILL        = 1016
	REASON_OPVP                      = 1017
	REASON_MEMOIR                    = 1018
	REASON_BUY_MEMOIR_TIMES          = 1019
	REASON_EQUIP_LEVEL_UP            = 1020
	REASON_RES_INSTANCE_BUY_TIMES    = 1021
	REASON_COMPOSE                   = 1022
	REASON_MACHINE_UNLOCK            = 1023
	REASON_MACHINE_LEVEL_UP          = 1024
	REASON_MACHINE_STAR_UP           = 1025
	REASON_MACHINE_REBORN            = 1026
	REASON_ARTIFACT_ADD_EXP          = 1027
	REASON_WORLD_BOSS_BUY_TIMES      = 1028
	REASON_BLACK_MARKET_BUY          = 1029
	REASON_BLACK_MARKET_REFRESH      = 1030
	REASON_MALL_PACKAGE              = 1031
	REASON_MALL_RECHARGE             = 1032
)
