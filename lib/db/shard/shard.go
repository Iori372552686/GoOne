// Package shard 提供按业务主键（uid）的分表路由规则。
//
// 定位：MySQL 侧（L3 持久层）的表名解析骨架。当前仅落地公式与规则对象，
// TableShards<=1 时表名不加后缀（与现状零差异）；>1 时生成 "{base}_{N}" 分表名。
// 分表数只增不改：扩容时保持旧表不动、新 uid 落新表，或经显式数据迁移——
// 公式一旦对外发布即为契约，rule_test.go 的表驱动用例用于防止无意变更。
//
// 本包不持有连接、不读配置（构造方负责从 conf 装配 Rule），保持纯函数可测。
package shard

import (
	"fmt"
	"regexp"
	"strings"
)

var identPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// RouteParams 每次路由的寻址参数。Uid 为分片键。
type RouteParams struct {
	Uid  uint64
	Zone string
	Env  string
}

// RouteTarget 路由结果：ORM 实例名（base_cfg.dependencies.orm_instances 的
// index_name）与物理表名（含分表后缀）。
type RouteTarget struct {
	Instance string
	Table    string
}

// Rule 一张逻辑表的分片规则。
type Rule struct {
	Name        string // 规则名，日志与监控用
	Instance    string // ORM 实例名，空 = "default"
	TableBase   string // 逻辑表名，如 "role_data"
	TableShards int    // 分表数，<=1 = 不分表（表名无后缀）
}

// Validate 校验规则字段。TableShards 不允许负数；Name/TableBase 必填，
// 且 TableBase 只允许 [A-Za-z0-9_]（它会拼进物理表名与 DDL）。
func (r *Rule) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("shard rule: name is required")
	}
	if strings.TrimSpace(r.TableBase) == "" {
		return fmt.Errorf("shard rule %q: table base is required", r.Name)
	}
	if !identPattern.MatchString(r.TableBase) {
		return fmt.Errorf("shard rule %q: table base %q contains illegal characters (allowed: [A-Za-z0-9_])", r.Name, r.TableBase)
	}
	if r.TableShards < 0 {
		return fmt.Errorf("shard rule %q: table shards must be >= 0, got %d", r.Name, r.TableShards)
	}
	return nil
}

// Resolve 解析路由目标。分表公式：fnv32a(uid) % TableShards。
// 公式是稳定契约：同一 uid 永远落同一张表，变更需携带数据迁移。
func (r *Rule) Resolve(rp RouteParams) (RouteTarget, error) {
	if err := r.Validate(); err != nil {
		return RouteTarget{}, err
	}
	instance := r.Instance
	if instance == "" {
		instance = "default"
	}
	table := r.TableBase
	if r.TableShards > 1 {
		table = fmt.Sprintf("%s_%d", r.TableBase, fnv32a(rp.Uid)%uint32(r.TableShards))
	}
	return RouteTarget{Instance: instance, Table: table}, nil
}

// fnv32a 对 uid 的 8 字节（小端序）做 FNV-1a 32 位哈希。
// 字节序与算法均为契约的一部分，不要改动——改动会使存量数据路由漂移。
func fnv32a(uid uint64) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for i := 0; i < 8; i++ {
		h ^= uint32(uid>>(8*i)) & 0xff
		h *= prime32
	}
	return h
}
