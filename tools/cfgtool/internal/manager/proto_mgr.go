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

	// externalProtoKeys 记录从 -proto-src 载入的外部 proto key。
	// 它们只参与类型解析（protoMgr/ParseProto），不应输出到 -proto 目录——
	// 否则会把 core/service/storage proto 复制进配置 proto 目录，
	// 后续 protoc 编译时与 core/ 原文件重复定义而失败。
	externalProtoKeys = make(map[string]struct{})
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
	externalProtoKeys = make(map[string]struct{})

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

// IsExternalProto 判断 key 是否来自 -proto-src 外部 proto 载入。
// 外部 proto 仅用于类型解析，SaveProto 不把它们输出到 -proto 目录。
func IsExternalProto(key string) bool {
	_, ok := externalProtoKeys[key]
	return ok
}

func GetProtoList() []string {
	return protoList
}

func GetProtoMap() map[string]string {
	return protoMgr
}

// LoadExternalProtos 加载 -proto-src 目录下所有 .proto 文件：
//  1. 读入 protoMgr（key 为相对根路径，如 core/struct.proto，与 import 路径风格一致），
//     使后续 ParseProto 能统一解析并 cross-link
//  2. 预解析每个文件，提取 message/enum 短名注册到 externalMsgMgr/externalEnumMgr
//
// 支持多目录（-proto-src 用路径分隔符拼接，Windows 为分号）：先加载全部目录的文件，
// 再统一预解析一次——import 图可能跨目录（如 service 协议 import 主仓 api/proto 的
// goone/options），逐目录解析会因依赖尚未加载而失败刷警告。
//
// key 为相对目录根的路径（去掉目录前缀，保证与 proto 内 import 路径一致）。
func LoadExternalProtos(dirs ...string) error {
	// 第一遍（所有目录）：读入 protoMgr，记录相对路径
	var relPaths []string
	for _, dir := range dirs {
		files, err := base.Glob(dir, `\.proto$`, true)
		if err != nil {
			return uerror.New(1, -1, "扫描外部proto目录失败: %v", err)
		}
		if len(files) == 0 {
			logx.Warnf("外部proto目录 %s 下无 .proto 文件", dir)
			continue
		}

		// 归一化目录（去掉末尾分隔符），用于计算相对路径
		srcRoot := filepath.Clean(dir)
		for _, abs := range files {
			rel, err := filepath.Rel(srcRoot, abs)
			if err != nil {
				rel = filepath.Base(abs)
			}
			// proto import 用正斜杠，统一转换
			rel = filepath.ToSlash(rel)
			// 跳过 config/（cfgtool 自己的 -proto 输出目录）：里面是上一次运行生成的
			// 配置 proto，本次会从 xlsx 重新生成同名文件，预加载会导致 ParseProto
			// 阶段出现重复符号定义。外部引用只需 core/storage/service 等源 proto。
			if strings.HasPrefix(rel, "config/") {
				continue
			}
			protoKey := rel
			content, err := os.ReadFile(abs)
			if err != nil {
				return uerror.New(1, -1, "读取外部proto失败 %s: %v", abs, err)
			}
			// 仅当 map 中不存在时写入（避免覆盖 cfgtool 自己生成的同名 key）
			if _, exists := protoMgr[protoKey]; !exists {
				protoMgr[protoKey] = string(content)
				protoList = append(protoList, protoKey)
				externalProtoKeys[protoKey] = struct{}{}
			}
			relPaths = append(relPaths, protoKey)
		}
	}
	if len(relPaths) == 0 {
		return nil
	}

	// 第二遍：预解析，提取 message/enum 短名注册。
	// 容错策略：先尝试一次性解析全部（性能最优，且能 cross-link 跨文件/跨目录的相互 import）；
	// 若整体失败（常见于部分 proto import 了仓库外文件如 google/protobuf/*.proto），
	// 回退为逐文件解析——成功的注册，失败的从 protoMgr 移除（避免污染后续主流程 ParseProto）。
	subMap := make(map[string]string, len(relPaths))
	for _, k := range relPaths {
		subMap[k] = protoMgr[k]
	}
	subParser := protoparse.Parser{Accessor: protoparse.FileContentsFromMap(subMap)}

	// 先尝试整体解析（支持跨文件 cross-link，如 struct.proto import role.proto）
	if descs, err := subParser.ParseFiles(relPaths...); err == nil {
		for i, k := range relPaths {
			if fd := descs[i]; fd != nil {
				registerExternalTypes(fd, k)
			}
		}
	} else {
		// 整体失败：回退为逐文件解析，容错跳过有外部依赖的 proto
		logx.Warnf("外部proto整体解析失败，回退为逐文件解析（部分含仓库外import的proto将被跳过）: %v", err)
		for _, k := range relPaths {
			d, perr := subParser.ParseFiles(k)
			if perr != nil {
				// 该文件依赖了 map 中没有的文件（如 google/protobuf/*.proto），
				// 从 protoMgr/protoList 移除，避免后续主流程 ParseProto 解析它时整体失败
				logx.Warnf("跳过外部proto %s: %v", k, perr)
				removeProto(k)
				continue
			}
			if d[0] != nil {
				registerExternalTypes(d[0], k)
			}
		}
	}
	logx.Infof("加载外部proto完成: %d 个文件, %d message, %d enum",
		len(protoList), len(externalMsgMgr), len(externalEnumMgr))
	return nil
}

// removeProto 从 protoMgr 与 protoList 中移除指定 proto 文件（用于容错清理）。
func removeProto(key string) {
	delete(protoMgr, key)
	out := protoList[:0]
	for _, k := range protoList {
		if k != key {
			out = append(out, k)
		}
	}
	protoList = out
}

// registerExternalTypes 把一个 FileDescriptor 里的顶层 message/enum 注册到外部注册表。
func registerExternalTypes(fd *desc.FileDescriptor, protoFile string) {
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
