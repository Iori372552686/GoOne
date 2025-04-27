package gamedata

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"io/ioutil"
	"path/filepath"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
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
		//dir, err := os.Getwd()
		//fmt.Println("---------->dir:", dir, err)
		buf, err := ioutil.ReadFile(filepath.Join(configureDir, fileName))
		if err != nil {
			return uerror.New(1, -1, err.Error())
		}

		if err := f(string(buf)); err != nil {
			return uerror.New(1, -1, "加载%s配置错误： %v", fileName, err)
		}
	}
	return nil
}

// 初始化配置中心
func InitNet(client config_client.IConfigClient, group string) error {
	for sheet, f := range fileMgr {
		fileName := sheet + ".conf"
		content, err := client.GetConfig(vo.ConfigParam{DataId: fileName, Group: group})
		if err != nil || content == "" {
			return uerror.New(1, -1, "nacos.GetConfig(%s): %v", fileName, err)
		}

		err = client.ListenConfig(vo.ConfigParam{
			DataId: fileName,
			Group:  group,
			OnChange: func(namespace, group, dataId, data string) {
				logger.Infof("gameconf changed !! ** update file: [group: %v , dataId: %v ] **", group, dataId)
				if err := f(data); err != nil {
					logger.Errorf("gameconf changed !! ** update file: [group: %v , dataId: %v ] **", group, dataId)
				}
			},
		})
		if err != nil {
			return uerror.New(1, -1, "nacos.ListenConfig(%s): %v", fileName, err)
		}

		if err := f(content); err != nil {
			return uerror.New(1, -1, "加载配置错误： %v", err)
		}
	}
	return nil
}
