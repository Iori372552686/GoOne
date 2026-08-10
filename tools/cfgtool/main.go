package main

/*
该工具从github个人仓库拷贝过来，临时使用 by Iori
*/

import (
	"flag"
	"fmt"
	"os"

	"path/filepath"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/errs"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/logx"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
	"github.com/Iori372552686/GoOne/tools/cfgtool/service"
)

func main() {
	verFlag := flag.Bool("version", false, "当前程序版本号")
	flag.StringVar(&domain.XlsxPath, "xlsx", "./xls", "cfg文件目录")
	flag.StringVar(&domain.TextPath, "text", "", "pb text数据文件目录")
	flag.StringVar(&domain.ProtoPath, "proto", "", "proto文件目录")
	flag.StringVar(&domain.ProtoSrcPath, "proto-src", "", "外部proto源文件目录（用于pb.XXX类型引用检索，通常指向 common/game_proto，使import路径与proto声明对齐）")
	flag.StringVar(&domain.JsonPath, "json", "", "json数据文件目录")
	flag.StringVar(&domain.BytesPath, "bytes", "", "pb bytes数据文件目录")
	flag.StringVar(&domain.LuaPath, "lua", "", "lua数据文件目录")
	flag.StringVar(&domain.TsPath, "ts", "", "ts数据文件目录")
	flag.StringVar(&domain.CodePath, "code", "", "go代码文件目录")
	flag.StringVar(&domain.CppPath, "cpp", "", "c++代码文件目录")
	flag.StringVar(&domain.NodeJsPath, "nodejs", "", "node.js/ts代码文件目录")
	flag.StringVar(&domain.ConfMode, "mode", "all", "配置gen模式（all：全部  client：客户端  server： 服务器）")
	flag.StringVar(&domain.Module, "module", "github.com/Iori372552686/GoOne", "项目目录")
	flag.StringVar(&domain.PbPath, "pb", "github.com/Iori372552686/g1_common/protocol", "proto生成路径")
	flag.StringVar(&domain.UploadURL, "upload", "", "配置中心URL（留空不上传；如 etcd://host:2379?path=/goone/config 或 nacos://host:8848?dataid=...&group=...）")
	flag.StringVar(&domain.UploadType, "uptype", "", "上传数据格式，逗号分隔（json/conf/bytes/lua），需配合对应 -json/-text/-bytes/-lua 目录")
	flag.Parse()

	if *verFlag {
		fmt.Println("Version:", domain.Version)
		os.Exit(0)
	}

	if len(domain.XlsxPath) <= 0 {
		logx.Errorf("%v", errs.New("", "", "", 0, "参数错误", "配置文件目录不能为空"))
		os.Exit(1)
	}
	if len(domain.PbPath) > 0 {
		domain.PkgName = filepath.Base(domain.PbPath)
	}
	// 执行主流程
	if err := run(); err != nil {
		logx.Errorf("%v", err)
		os.Exit(1)
	}
}

func init() {
	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Fprint(flag.CommandLine.Output(), fmt.Sprintf(`
		命名约定（功能名@表名 体系）：
		  xlsx 文件名: <中文名>@<功能名>.xlsx   例如 掉落系统@drop.xlsx  -> 功能名=drop
		  sheet    名: <中文名>@<表名>          例如 掉落组表@group      -> 表名=group
		  普通配置表可不写生成表，工具会自动把数据 sheet 注册为 config。
		  派生命名：类型名=DropGroupConfig / proto=drop.proto / Go文件=gdata_drop_group.go / 子目录=drop

		枚举类型说明：
		E|道具类型-金币|PropertType|Coin|1	
		
		配置规则说明（生成表中填写；配置了生成表则覆盖默认 sheet 规则）：
		@config|sheet:结构名|map:[字段名,字段名]:别名|lua:[args,args2]
		@struct|sheet:结构名
		@enum|sheet
		`))
	}
}

func run() error {
	// 加载所有配置
	files, err := base.Glob(domain.XlsxPath, ".*\\.xlsx", true)
	if err != nil {
		return errs.Wrap(err, "", "", "", 0, "加载错误", "扫描xlsx目录失败")
	}
	// 预加载外部 proto（必须在 ParseFiles 之前，使 buildField 能识别 pb.XXX 类型、
	// parseReference 能收集外部 import）。留空则跳过。
	// -proto-src 支持多目录（按系统路径分隔符拼接，Windows 用分号），
	// 例如 common/game_proto;api/proto —— service 协议 import 的 goone/options 在主仓。
	if len(domain.ProtoSrcPath) > 0 {
		if err := manager.LoadExternalProtos(filepath.SplitList(domain.ProtoSrcPath)...); err != nil {
			return errs.Wrap(err, "", "", "", 0, "加载错误", "加载外部proto失败")
		}
	}
	// 解析所有文件
	if err := parser.ParseFiles(files...); err != nil {
		return errs.Wrap(err, "", "", "", 0, "解析错误", "解析xlsx失败")
	}
	// 生成proto文件数据
	if err := service.GenProto(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "生成错误", "生成proto数据失败")
	}
	if err := service.SaveProto(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "保存错误", "保存proto文件失败")
	}
	// 解析proto文件
	if err := manager.ParseProto(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "解析错误", "解析proto描述符失败")
	}
	// 生成数据
	if err := service.GenData(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "生成错误", "生成配置数据失败")
	} else {
		logx.Successf("配置数据生成完成")
	}
	// 上传生成产物到配置中心（可选；-upload 为空则跳过）
	if err := service.UploadData(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "上传错误", "上传配置数据失败")
	}
	// 生成code代码（可选）
	if err := service.GenCode(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "生成错误", "生成Go代码失败")
	}
	// 生成C++代码（可选）
	if err := service.GenCpp(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "生成错误", "生成C++代码失败")
	}
	// 生成Node.js/TypeScript代码（可选）
	if err := service.GenNodeJs(); err != nil {
		return errs.Wrap(err, "", "", "", 0, "生成错误", "生成Node.js代码失败")
	}

	// 所有 Gen* 完成后统一清理内存表（原先在 GenData 末尾清理，会令后续 GenCode/GenCpp/GenNodeJs 拿到空表）
	manager.Clear()

	logx.Successf("当前gen模式[%s]:所有的执行已顺利完成,恭喜 ^_^ ！", domain.ConfMode)
	return nil
}
