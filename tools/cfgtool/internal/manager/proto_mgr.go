package manager

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/logx"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
)

var (
	referenceMgr = make(map[string][]string)
	protoMgr     = make(map[string]string)
	protoList    = []string{}
	descMap      = make(map[string]*desc.FileDescriptor)

	// 外部 proto 类型注册表：短名 -> 元信息
	// 与 structMgr/enumMgr 平行，专门存放从 -proto-src 目录加载的 message/enum。
	// pb.XXX 语法解析后，XXX 在此查表。
	externalMsgMgr  = make(map[string]*ExternalType)
	externalEnumMgr = make(map[string]*ExternalType)
)

// ExternalType 描述一个外部 proto 定义的 message 或 enum。
// FullName 是全限定名（如 g1.protocol.Reward），Name 是短名（Reward），
// ProtoFile 是该类型所在 proto 文件的「相对仓库根路径」（如 proto/core/struct.proto），
// 用于生成 import 语句。
type ExternalType struct {
	Name      string
	FullName  string
	ProtoFile string
}

// Clear resets every manager-global container so that sequential test cases
// (or repeated runs) start from a clean slate. Maps are re-initialized to empty
// (not nil) because the various Add* helpers write to them without a nil check.
//
// 注意：convertMgr 不在此清理之列——它由 init() 注册基础标量类型
// （int32/string/bool 等）的转换器，是静态表而非累积状态，清空会导致
// 所有基础类型被识别为"未识别的类型"。
func Clear() {
	// proto_mgr
	referenceMgr = make(map[string][]string)
	protoMgr = make(map[string]string)
	protoList = nil
	descMap = make(map[string]*desc.FileDescriptor)
	externalMsgMgr = make(map[string]*ExternalType)
	externalEnumMgr = make(map[string]*ExternalType)

	// table_mgr
	tableMgr = make(map[string]*base.Table)
	groupMgr = make(map[int][]*base.Table)

	// type_mgr
	configMgr = make(map[string]*base.Config)
	structMgr = make(map[string]*base.Struct)
	enumMgr = make(map[string]*base.Enum)
}

func AddRef(filename string, reference map[string]struct{}) {
	for ke := range reference {
		referenceMgr[filename] = append(referenceMgr[filename], ke)
	}
}

func GetRefList(file string) []string {
	refs := referenceMgr[file]
	unique := make(map[string]struct{}, len(refs))
	var res []string
	for _, v := range refs {
		if _, ok := unique[v]; !ok {
			unique[v] = struct{}{}
			res = append(res, v)
		}
	}

	return res
}

func AddProto(file string, buf *bytes.Buffer) {
	filename := base.GetProtoName(file)
	protoMgr[filename] = buf.String()
	protoList = append(protoList, filename)
}

func GetProtoList() []string {
	return protoList
}

func GetProtoMap() map[string]string {
	return protoMgr
}

// LoadExternalProtos 加载 -proto-src 目录下所有 .proto 文件：
//  1. 读入 protoMgr（key 为相对根路径，如 proto/core/struct.proto，与 import 路径风格一致），
//     使后续 ParseProto 能统一解析并 cross-link
//  2. 预解析每个文件，提取 message/enum 短名注册到 externalMsgMgr/externalEnumMgr
//
// protoFileRelPath 用于把磁盘绝对路径转换为相对仓库根的 proto 路径（去掉 ProtoSrcPath 前缀，
// 保证 import 路径与 protoMgr 的 key 对齐）。留空则用文件名。
func LoadExternalProtos(dir string) error {
	files, err := base.Glob(dir, `\.proto$`, true)
	if err != nil {
		return uerror.New(1, -1, "扫描外部proto目录失败: %v", err)
	}
	if len(files) == 0 {
		logx.Warnf("外部proto目录 %s 下无 .proto 文件", dir)
		return nil
	}

	// 归一化目录（去掉末尾分隔符），用于计算相对路径
	srcRoot := filepath.Clean(dir)

	// 第一遍：读入 protoMgr，并记录「相对路径 -> 绝对路径」
	relToAbs := make(map[string]string, len(files))
	var relPaths []string
	for _, abs := range files {
		rel, err := filepath.Rel(srcRoot, abs)
		if err != nil {
			rel = filepath.Base(abs)
		}
		// proto import 用正斜杠，统一转换
		rel = filepath.ToSlash(rel)
		// 外部 proto 的统一路径前缀：proto/...
		protoKey := rel
		content, err := os.ReadFile(abs)
		if err != nil {
			return uerror.New(1, -1, "读取外部proto失败 %s: %v", abs, err)
		}
		// 仅当 map 中不存在时写入（避免覆盖 cfgtool 自己生成的同名 key）
		if _, exists := protoMgr[protoKey]; !exists {
			protoMgr[protoKey] = string(content)
			protoList = append(protoList, protoKey)
		}
		relToAbs[protoKey] = abs
		relPaths = append(relPaths, protoKey)
	}

	// 第二遍：预解析，提取 message/enum 短名注册
	// 用一个独立 parser（只解析外部 proto 子集），避免影响主 ParseProto 流程
	subMap := make(map[string]string, len(relPaths))
	for _, k := range relPaths {
		subMap[k] = protoMgr[k]
	}
	subParser := protoparse.Parser{Accessor: protoparse.FileContentsFromMap(subMap)}
	descs, err := subParser.ParseFiles(relPaths...)
	if err != nil {
		return uerror.New(1, -1, "解析外部proto失败: %s", err.Error())
	}
	for i, k := range relPaths {
		fd := descs[i]
		if fd == nil {
			continue
		}
		registerExternalTypes(fd, k)
	}
	logx.Infof("加载外部proto完成: %d 个文件, %d message, %d enum",
		len(relPaths), len(externalMsgMgr), len(externalEnumMgr))
	return nil
}

// registerExternalTypes 把一个 FileDescriptor 里的顶层 message/enum 注册到外部注册表。
func registerExternalTypes(fd *desc.FileDescriptor, protoFile string) {
	pkg := fd.GetPackage() // 通常为 g1.protocol
	for _, mt := range fd.GetMessageTypes() {
		short := mt.GetName()
		full := mt.GetFullyQualifiedName()
		externalMsgMgr[short] = &ExternalType{
			Name:      short,
			FullName:  full,
			ProtoFile: protoFile,
		}
	}
	for _, et := range fd.GetEnumTypes() {
		short := et.GetName()
		full := et.GetFullyQualifiedName()
		externalEnumMgr[short] = &ExternalType{
			Name:      short,
			FullName:  full,
			ProtoFile: protoFile,
		}
	}
	_ = pkg
}

// ----- 外部类型检索 -----

func GetExternalMsg(name string) *ExternalType {
	return externalMsgMgr[name]
}

func GetExternalEnum(name string) *ExternalType {
	return externalEnumMgr[name]
}

func IsExternalMsg(name string) bool {
	_, ok := externalMsgMgr[name]
	return ok
}

func IsExternalEnum(name string) bool {
	_, ok := externalEnumMgr[name]
	return ok
}

func ParseProto() error {
	paser := protoparse.Parser{Accessor: protoparse.FileContentsFromMap(protoMgr)}
	descs, err := paser.ParseFiles(protoList...)
	if err != nil {
		return uerror.New(1, -1, "parse proto file error: %s", err.Error())
	}
	for i := range protoList {
		descMap[protoList[i]] = descs[i]
	}
	return nil
}

// NewProto 按 message 名构造 dynamic.Message。
// 检索策略：
//  1. 先在 fileName 对应的 FileDescriptor 上用全限定名查找（内置 struct/config 的常规路径）
//  2. 找不到则全局遍历所有 descMap（外部 message 可能定义在任意外部 proto 文件）
func NewProto(fileName, name string) *dynamic.Message {
	fullName := domain.ProtoPkgName + "." + name
	// 路径 1：指定文件内查找
	if val, ok := descMap[base.GetProtoName(fileName)]; ok {
		if typeOf := val.FindMessage(fullName); typeOf != nil {
			return dynamic.NewMessage(typeOf)
		}
	}
	// 路径 2：外部 message 全局查找——遍历所有 FileDescriptor
	for _, fd := range descMap {
		if typeOf := fd.FindMessage(fullName); typeOf != nil {
			return dynamic.NewMessage(typeOf)
		}
	}
	// 路径 3：用 ExternalType 的 FullName 兜底（跨 package 极少数情况）
	if et, ok := externalMsgMgr[name]; ok {
		for _, fd := range descMap {
			if typeOf := fd.FindMessage(et.FullName); typeOf != nil {
				return dynamic.NewMessage(typeOf)
			}
		}
	}
	return nil
}

// FindExternalMsgDescriptor 返回外部 message 的 MessageDescriptor（供数据生成反射用）。
func FindExternalMsgDescriptor(name string) *desc.MessageDescriptor {
	if et, ok := externalMsgMgr[name]; ok {
		for _, fd := range descMap {
			if mt := fd.FindMessage(et.FullName); mt != nil {
				return mt
			}
		}
	}
	return nil
}

// 兼容：抑制 strings 在部分构建路径下未使用的告警
var _ = strings.TrimSpace
