package zip

import (
	"bytes"
	"compress/zlib"
	"io"
)

/**
* @Description: encode
* @param: data
* @return: []byte
* @Author: Iori
**/
func ZipEncode(data []byte) []byte {
	var in bytes.Buffer

	z := zlib.NewWriter(&in)
	z.Write(data)
	z.Close()
	return in.Bytes()
}

/**
* @Description:  string encode
* @param: data
* @return: string
* @Author: Iori
* @Date: 2022-06-01 15:12:16
**/
func ZipEncodeStr(data string) string {
	var in bytes.Buffer

	z := zlib.NewWriter(&in)
	z.Write([]byte(data))
	z.Close()
	return in.String()
}

/**
* @Description:  decode
* @param: data
* @return: []byte
* @Author: Iori
* @Date: 2022-06-01 15:10:03
**/
func ZipDecode(data []byte) ([]byte, error) {
	var out bytes.Buffer
	var in bytes.Buffer

	in.Write(data)
	r, err := zlib.NewReader(&in)
	if err != nil {
		return nil, err
	}

	r.Close()
	io.Copy(&out, r)

	return out.Bytes(), nil
}

func ZipDecodeStr(data string) string {
	var out bytes.Buffer
	var in bytes.Buffer

	in.Write([]byte(data))
	r, _ := zlib.NewReader(&in)
	r.Close()
	io.Copy(&out, r)
	return out.String()
}
