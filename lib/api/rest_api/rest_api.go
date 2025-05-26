package rest_api

import (
	"errors"
	"github.com/Iori372552686/GoOne/lib/api/http_sign"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/convert"
	http_client "github.com/Iori372552686/GoOne/lib/web/http"
	"math/rand"
)

/**
 * RestApi
 * @Description:
**/
type RestApi struct {
	serviceName string              //服务名
	urlAddr     *UrlConfig          //url addrs
	signName    string              //sign indexName
	signImpl    *http_sign.HttpSign //sign impl

	//private
	user     string
	password string
}

/**
* @Description: new  restapi impl
* @param: conf
* @return: *RestApi
* @Author: Iori
* @Date: 2022-07-06 15:14:20
**/
func NewRestInstances(conf Config, signs *http_sign.SignMgr) *RestApi {
	//check args
	if conf.ServiceName == "" || len(conf.Urls) == 0 {
		return nil
	}

	//new obj
	impl := &RestApi{}
	impl.serviceName = conf.ServiceName
	impl.urlAddr = NewUrlConf(conf.Urls)
	impl.user = conf.User
	impl.password = conf.Pass
	impl.signName = conf.SignName
	if signs != nil && conf.SignName != "" {
		impl.signImpl = signs.GetSignIns(conf.SignName)
	}

	logger.Infof("[%v] RestIns Init. ", conf.ServiceName)
	return impl
}

/**
* @Description:  创建url实例
* @param: urls
* @return: config
* @Author: Iori
* @Date: 2022-07-06 15:12:21
**/
func NewUrlConf(urls []string) (config *UrlConfig) {
	count := len(urls)
	if count <= 0 {
		return nil
	}

	config = &UrlConfig{Urls: urls, UrlCount: int64(count)}
	return
}

/**
* @Description: 根据uin hash获取
* @param: uin
* @return: string
* @Author: Iori
* @Date: 2022-07-06 15:12:24
**/
func (self *UrlConfig) GetHashUrl(uin ...int64) string {
	if self.UrlCount == 1 {
		return self.Urls[0]
	}

	if uin == nil || uin[0] == 0 {
		uin = make([]int64, 1)
		uin[0] = int64(rand.Intn(int(self.UrlCount)))
	}

	return self.Urls[uin[0]%self.UrlCount]
}

//--------------------------------------- func -------------------

/**
* @Description: 带签名的get请求
* @param: uin
* @param: uriMap
* @return: map[string]interface{}
* @return: error
* @Author: Iori
* @Date: 2022-07-06 17:11:14
**/
func (self *RestApi) SignGet(uin int64, uriMap *map[string]string) ([]byte, error) {
	if self == nil || self.signImpl == nil {
		return nil, errors.New("signImpl  is nil ,not signReq !")
	}

	url := self.urlAddr.GetHashUrl(uin) + http_sign.Map2uri(self.signImpl.PushSign(uriMap, nil, http_sign.Sign_Md5), "", true, false)
	rspBody, err := http_client.HttpGetRequest(url, "")
	if err != nil {
		logger.Errorf("SignGet Request err | %v", err.Error())
		return nil, err
	}

	return rspBody, nil
}

/**
* @Description: get请求
* @param: uin
* @param: uriMap
* @return: map[string]interface{}
* @return: error
* @Author: Iori
* @Date: 2022-07-06 17:11:14
**/
func (self *RestApi) Get(uin int64, uriMap *map[string]string) ([]byte, error) {
	if self == nil {
		return nil, errors.New("impl is nil ,not Get !")
	}

	//req get
	url := self.urlAddr.GetHashUrl(uin) + http_sign.Map2uri(uriMap, "", true, false)
	rspBody, err := http_client.HttpGetRequest(url, "")
	if err != nil {
		logger.Errorf("Get Request err | %v", err.Error())
		return convert.Str2bytes(url), err
	}

	return rspBody, nil
}

/**
* @Description: 带签名的post,新规范请求
* @param: uin
* @param: uriMap
* @return: map[string]interface{}
* @return: error
* @Author: Iori
* @Date: 2022-07-06 17:11:32
**/
func (self *RestApi) SignPost(common_param, actions *map[string]interface{}) ([]byte, error) {
	if self == nil || self.signImpl == nil || actions == nil {
		return nil, errors.New("SignImpl or actions  is nil ,not signReq !")
	}

	uin := int64(0)
	uriMap := make(map[string]string)
	if common_param != nil && (*common_param)["uin"] != nil {
		uin = (*common_param)["uin"].(int64)
	}

	//gen body
	bodystr := convert.StructToJson(&map[string]interface{}{"common_param": common_param, "actions": actions})
	rspBody, err := http_client.HttpPostRequest(self.urlAddr.GetHashUrl(uin)+
		http_sign.Map2uri(self.signImpl.PushSign(&uriMap, bodystr, http_sign.Sign_Md5), "", true, false), convert.Bytes2str(bodystr))
	if err != nil {
		logger.Errorf("SignPost Request err | %v", err.Error())
		return nil, err
	}

	return rspBody, nil
}

/**
* @Description: post新规范请求
* @param: uin
* @param: uriMap
* @return: map[string]interface{}
* @return: error
* @Author: Iori
* @Date: 2022-07-06 17:11:32
**/
func (self *RestApi) Post(common_param, actions *map[string]interface{}) ([]byte, error) {
	if self == nil || actions == nil {
		return nil, errors.New("body is nil,not Post Req !")
	}

	uin := int64(0)
	if common_param != nil && (*common_param)["uin"] != nil {
		uin = int64((*common_param)["uin"].(float64))
	}

	//gen body
	bodystr := convert.StructToJsonStr(&map[string]interface{}{"common_param": common_param, "actions": actions})
	rspBody, err := http_client.HttpPostRequest(self.urlAddr.GetHashUrl(uin), bodystr)
	if err != nil {
		logger.Errorf("Post Request err | %v", err.Error())
		return nil, err
	}

	return rspBody, nil
}

/**
* @Description: 带签名的post,不带规范
* @param: uin
* @param: uriMap
* @return: map[string]interface{}
* @return: error
* @Author: Iori
* @Date: 2025-03-15 17:11:32
**/
func (self *RestApi) SignPostV2(headMap, uriMap *map[string]string, actions *map[string]interface{}) ([]byte, error) {
	if self == nil || self.signImpl == nil || actions == nil {
		return nil, errors.New("SignImpl or actions  is nil ,not signReq !")
	}

	uid := int64(0)
	if uriMap != nil && (*uriMap)["uid"] != "" {
		uid = int64(convert.StrToInt((*uriMap)["uid"]))
	}

	//gen body
	bodystr := convert.StructToJson(actions)
	rspBody, err := http_client.HeaderHttpPostRequest(self.urlAddr.GetHashUrl(uid)+http_sign.Map2uri(self.signImpl.PushSign(uriMap, bodystr, http_sign.Sign_Md5), "", true, false),
		convert.Bytes2str(bodystr), headMap)
	if err != nil {
		logger.Errorf("SignPost Request err | %v", err.Error())
		return nil, err
	}

	return rspBody, nil
}
