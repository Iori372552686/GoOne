package http_client

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// http args
var HttpConnectPool http.RoundTripper = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          1000,
	IdleConnTimeout:       60 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true,
	},
}

/**
* @Description:
* @param: url
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-02-15 17:16:28
**/
func GetRequest(url string) ([]byte, error) {
	logger.Infof(" GetRequest -- url:%v", url)
	return HttpRequest("GET", url, "")
}

/**
* @Description:  http请求 ，支持传入方式
* @param: method
* @param: url
* @param: requestBody
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-02-15 17:16:03
**/
func HttpRequest(method string, url, requestBody string) ([]byte, error) {
	client := &http.Client{
		Transport: HttpConnectPool,
	}

	req, err := http.NewRequest(method, url, strings.NewReader(requestBody))
	if err != nil {
		logger.Errorf("http Request err: %v", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client.Timeout = 8 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("client do err: %v", err)
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logger.Errorf("Error response.StatusCode=%v urlstr=%s", resp.StatusCode, url)
		return nil, errors.New("resp.Code != 200")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, err
}

/**
* @Description: 带token的http请求
* @param: method
* @param: value
* @param: url
* @param: token
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-02-15 17:16:33
**/
func TokenHttpRequest(method string, value url.Values, url, token string) ([]byte, error) {
	client := &http.Client{
		Transport: HttpConnectPool,
	}
	requestBody := value.Encode()

	req, err := http.NewRequest(method, url, strings.NewReader(requestBody))
	if err != nil {
		logger.Errorf("http post err: %v", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", token)

	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("client do err: %v", err)
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logger.Errorf("Error response.StatusCode=%v urlstr=%s", resp.StatusCode, url)
		return nil, errors.New("resp.Code != 200")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, err
}

/**
* @Description: 不带授权get请求
* @param: urlstr
* @param: reqBody
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-02-15 17:16:41
**/
func HttpGetRequest(urlstr string, reqBody string) ([]byte, error) {
	client := &http.Client{
		Transport: HttpConnectPool,
	}
	request, e := http.NewRequest("GET", urlstr, strings.NewReader(reqBody))
	if e != nil {
		return nil, e
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Error response.StatusCode=%v urlstr=%s", response.StatusCode, urlstr)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, err
}

/**
* @Description: 不带授权post请求
* @param: urlstr
* @param: msgbody
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-02-15 17:17:05
**/
func HttpPostRequest(urlstr string, msgbody string) ([]byte, error) {
	client := &http.Client{
		Transport: HttpConnectPool,
	}

	logger.Debugf("HttpPostRequest url=%v | body= %v", urlstr, msgbody)
	request, e := http.NewRequest("POST", urlstr, strings.NewReader(msgbody))
	request.Header.Set("Content-type", "application/json")
	if e != nil {
		return nil, e
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Error response.StatusCode=%v urlstr=%s", response.StatusCode, urlstr)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, err
}

/**
* @Description: 自定义head的post请求
* @param: urlstr
* @param: msgbody
* @param: headers
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-02-15 17:17:05
**/
func HeaderHttpPostRequest(urlstr string, msgbody string, headers *map[string]string) ([]byte, error) {
	client := &http.Client{
		Transport: HttpConnectPool,
	}
	request, e := http.NewRequest("POST", urlstr, strings.NewReader(msgbody))
	request.Header.Set("Content-type", "application/json")
	for k, v := range *headers {
		request.Header.Set(k, v)
	}
	if e != nil {
		return nil, e
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 && response.StatusCode != 201 {
		return body, fmt.Errorf("Error response.StatusCode=%v urlstr=%s", response.StatusCode, urlstr)
	}

	return body, err
}
