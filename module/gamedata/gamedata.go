package gamedata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/uerror"
	contribconfig "github.com/Iori372552686/GoOne/lib/contrib/config"
)

var (
	configureDir string
	fileMgr      = make(map[string]func(string) error)
)

func Register(sheet string, f func(string) error) {
	if _, ok := fileMgr[sheet]; ok {
		panic(fmt.Sprintf("%s已经注册过了", sheet))
	}
	fileMgr[sheet] = f
}

func InitLocal(dir string) error {
	configureDir = dir

	for sheet, f := range fileMgr {
		fileName := sheet + ".conf"
		// 加载整个文件
		buf, err := os.ReadFile(filepath.Join(configureDir, fileName))
		if err != nil {
			return uerror.New(1, -1, "%s", err.Error())
		}

		if err := f(string(buf)); err != nil {
			return uerror.New(1, -1, "加载%s配置错误： %v", fileName, err)
		}
	}
	return nil
}

// SheetFiles 返回按字典序排列的远端配置文件名（"<sheet>.conf"），
// 供 app init 时构造 lib/contrib/config/factory 的 DataIDs。
func SheetFiles() []string {
	sheets := sortedSheets()
	files := make([]string, 0, len(sheets))
	for _, s := range sheets {
		files = append(files, s+".conf")
	}
	return files
}

// remoteState 持有远端配置的运行时状态：contrib config client、watcher 与热更 goroutine。
//
// 历史缺陷：旧 InitNet 直接用 nacos ListenConfig 且不保存句柄，监听 goroutine
// 在进程生命周期内泄漏，也无法取消。现统一走 lib/contrib/config 抽象（Source/Watch），
// 后端由 lib/contrib/config/factory 在 app init 时构造（nacos/etcd/consul/k8s/apollo 均可）。
type remoteState struct {
	cli     contribconfig.Client
	watcher contribconfig.Watcher
	done    chan struct{} // 热更 goroutine 退出信号
}

var (
	remoteMu sync.Mutex
	remote   *remoteState
)

// InitRemote 基于 contrib config Client 加载全部表并启动热更监听。
//
// 语义：
//   - Load 一次性拉取所有 DataID，按字典序逐表解析（消除 map 迭代随机性的跨表混合版本窗口）；
//   - 初始化阶段严格：任一表缺失/为空/解析失败即整体失败，不启动 watcher；
//   - 热更阶段宽松：单表解析失败仅记日志，parse 不 Store，该表旧数据仍完整可用
//     （每表 atomic.Value 由生成代码保证）；
//   - 成功后接管 cli 生命周期：StopNet 停止 watcher、等待热更 goroutine 退出并 Close cli；
//     失败时不接管（调用方负责 Close）。
func InitRemote(cli contribconfig.Client) error {
	if cli == nil {
		return uerror.New(1, -1, "gamedata.InitRemote: config client is nil")
	}
	sheets := sortedSheets()
	if len(sheets) == 0 {
		return uerror.New(1, -1, "gamedata.InitRemote: no sheets registered")
	}

	kvs, err := cli.Load()
	if err != nil {
		return uerror.New(1, -1, "gamedata.InitRemote load: %v", err)
	}
	if err := applyKVs(kvs, sheets, true); err != nil {
		return err
	}

	watcher, err := cli.Watch()
	if err != nil {
		return uerror.New(1, -1, "gamedata.InitRemote watch: %v", err)
	}

	st := &remoteState{cli: cli, watcher: watcher, done: make(chan struct{})}
	go st.loop(sheets)

	remoteMu.Lock()
	if remote != nil {
		// 重复初始化：先回收旧 watcher/goroutine/client，避免泄漏。
		remote.stop()
	}
	remote = st
	remoteMu.Unlock()
	return nil
}

// StopNet 停止远端配置热更并释放 config client。幂等。
// 用于服务关闭时回收监听 goroutine 与后端连接，避免泄漏。
func StopNet() {
	remoteMu.Lock()
	st := remote
	remote = nil
	remoteMu.Unlock()
	if st != nil {
		st.stop()
	}
}

// loop 热更循环：阻塞等变更事件，解析失败仅记日志（旧数据保留）。
// watcher.Stop 后 Next 返回 error（context.Canceled），循环退出并关闭 done。
func (st *remoteState) loop(sheets []string) {
	defer close(st.done)
	for {
		kvs, err := st.watcher.Next()
		if err != nil {
			logger.Infof("gamedata remote watch exit | %v", err)
			return
		}
		// 热更宽松模式：单表失败在 applyKVs 内逐表记日志，旧数据保留。
		_ = applyKVs(kvs, sheets, false)
	}
}

// stop 停止 watcher、等待热更 goroutine 退出、关闭底层 client。
func (st *remoteState) stop() {
	_ = st.watcher.Stop()
	<-st.done
	_ = st.cli.Close()
}

// applyKVs 按 sheet 字典序应用一批 KeyValue。
// strict=true（初始化）：表缺失/为空/解析失败即返回 error；
// strict=false（热更）：缺失/为空跳过，解析失败逐表记日志（该表旧数据保留），整体返回 nil。
func applyKVs(kvs []*contribconfig.KeyValue, sheets []string, strict bool) error {
	byKey := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv == nil {
			continue
		}
		byKey[kv.Key] = string(kv.Value)
	}

	for _, sheet := range sheets {
		fileName := sheet + ".conf"
		content, ok := byKey[fileName]
		if !ok || strings.TrimSpace(content) == "" {
			if strict {
				return uerror.New(1, -1, "远端配置缺失或为空: %s", fileName)
			}
			continue
		}
		if err := fileMgr[sheet](content); err != nil {
			if strict {
				return uerror.New(1, -1, "加载%s配置错误： %v", fileName, err)
			}
			logger.Errorf("gameconf changed parse failed! ** [dataId: %v ] err: %v **", fileName, err)
		}
	}
	return nil
}

// sortedSheets 返回按字典序排列的 sheet 名，消除 map 迭代随机性带来的跨表混合版本窗口
// 。
func sortedSheets() []string {
	sheets := make([]string, 0, len(fileMgr))
	for sheet := range fileMgr {
		sheets = append(sheets, sheet)
	}
	// 简单插入排序，避免为小集合引入 sort 依赖开销（sheet 数量有限）。
	for i := 1; i < len(sheets); i++ {
		for j := i; j > 0 && sheets[j-1] > sheets[j]; j-- {
			sheets[j-1], sheets[j] = sheets[j], sheets[j-1]
		}
	}
	return sheets
}
