package manager

import (
	"bytes"
	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
)

var (
	referenceMgr = make(map[string][]string)
	protoMgr     = make(map[string]string)
	protoList    = []string{}
	descMap      = make(map[string]*desc.FileDescriptor)
)

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

func NewProto(fileName, name string) *dynamic.Message {
	val, ok := descMap[base.GetProtoName(fileName)]
	if !ok {
		return nil
	}
	typeOf := val.FindMessage(domain.ProtoPkgName + "." + name)
	if typeOf == nil {
		return nil
	}
	return dynamic.NewMessage(typeOf)
}
