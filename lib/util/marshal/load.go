package marshal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

/**
* @Description: 根据文件名,匹配解析配置协议
* @param: fileName
* @param: object
* @return: error
* @Author: Iori
* @Date: 2022-07-26 17:32:38
**/
func LoadConfFile(fileName string, object interface{}) error {
	var err error

	switch filepath.Ext(strings.ToLower(fileName)) {
	case ".json":
		err = LoadJson(fileName, object)
	case ".yaml", ".yml":
		err = LoadYaml(fileName, object)
	default:
		err = LoadJson(fileName, object)
	}

	return err
}

/**
* @Description: 加载并解析json文件
* @param: filePath
* @param: object
* @return: error
* @Author: Iori
* @Date: 2022-02-15 15:11:09
**/
func LoadJson(filePath string, object interface{}) error {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v | %w", filePath, err)
	}

	err = json.Unmarshal(contents, object)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config: %v | %w", filePath, err)
	}

	return nil
}

/**
* @Description: 加载并解析yaml文件
* @param: filePath
* @param: object
* @return: error
* @Author: Iori
* @Date: 2022-02-15 15:11:42
**/
func LoadYaml(filePath string, object interface{}) error {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v | %w", filePath, err)
	}

	err = yaml.Unmarshal(contents, object)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config: %v | %w", filePath, err)
	}

	return nil
}
