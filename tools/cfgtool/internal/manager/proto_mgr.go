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

func Clear() {
	referenceMgr = nil
	protoMgr = nil
	protoList = nil
	descMap = nil
}

func AddRef(filename string, reference map[string]struct{}) {
	for ke := range reference {
		referenceMgr[filename] = append(referenceMgr[filename], ke)
	}
}

func GetRefList(file string) []string {
	return referenceMgr[file]
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
