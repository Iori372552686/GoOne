package zip

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
)

/**
* @Description:  gzip Encode
* @param: in
* @return: string
* @return: error
* @Author: Iori
* @Date: 2022-05-31 21:38:05
**/
func GzipEncode(in []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)

	_, err := writer.Write(in)
	if err != nil {
		writer.Close()
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

/**
* @Description:
* @param: in
* @return: string
* @return: error
* @Author: Iori
* @Date: 2022-06-01 16:34:42
**/
func GzipEncodeStr(in string) (string, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)

	_, err := writer.Write([]byte(in))
	if err != nil {
		writer.Close()
		return "", err
	}
	err = writer.Close()
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

/**
* @Description:  gzip Decode
* @param: in
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-01 15:09:22
**/
func GzipDecode(in []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewBuffer(in))
	if err != nil {
		return nil, err
	}

	defer reader.Close()
	return ioutil.ReadAll(reader)
}

/**
* @Description: Gzip 旧版本，少尾部8byte
* @param: data
* @return: string
* @Author: Iori
* @Date: 2022-06-15 18:35:52
**/
func Gzip(data []byte) string {
	gzipBuff := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(gzipBuff)
	defer gzipWriter.Close()

	bytes.NewBuffer(data).WriteTo(gzipWriter)
	gzipWriter.Flush()

	return gzipBuff.String()
}

/**
* @Description:  Gzip 旧版本，少尾部8byte
* @param: data
* @return: string
* @return: error
* @Author: Iori
* @Date: 2022-06-15 18:35:50
**/
func UnGzip(data []byte) ([]byte, error) {
	dest := bytes.NewBuffer(nil)
	gzipBuff := bytes.NewBuffer(data)
	reader, err := gzip.NewReader(gzipBuff)
	if err != nil {
		return nil, err
	}

	defer reader.Close()
	io.Copy(dest, reader)

	return dest.Bytes(), nil
}
