package xxtea

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/rand"
)

const delta = 0x9e3779b9

func mx(y, z, p, e, sum uint32, key []uint32) uint32 {
	return ((z>>5 ^ y<<2) + (y>>3 ^ z<<4)) ^ ((sum ^ y) + (key[(p&3)^e] ^ z))
}

/**
* @Description:  btea 算法
* @param: v
* @param: n
* @param: key
* @param: rounds
* @Author: Iori
* @Date: 2022-06-13 20:14:25
**/
func btea(v []uint32, n int, key []uint32, rounds uint32) {
	var i, y, z, p, e, sum uint32
	// Coding Part
	if n > 1 {
		un := uint32(n)
		if rounds == 0 {
			rounds = 6 + 52/un
		}
		z = v[n-1]

		for {
			sum += delta
			e = (sum >> 2) & 3
			for p = 0; p < un-1; p++ {
				y = v[p+1]
				v[p] += mx(y, z, p, e, sum, key)
				z = v[p]
			}

			y = v[0]
			v[n-1] += mx(y, z, p, e, sum, key)
			z = v[n-1]

			i++
			if i > rounds-1 {
				break
			}
		}

	} else if n < -1 { // Decoding Part
		un := uint32(-n)
		if rounds == 0 {
			rounds = 6 + 52/un
		}

		sum = rounds * delta
		y = v[0]

		for {
			e = (sum >> 2) & 3
			for p = un - 1; p > 0; p-- {
				z = v[p-1]
				v[p] -= mx(y, z, p, e, sum, key)
				y = v[p]
			}

			z = v[un-1]
			v[0] -= mx(y, z, p, e, sum, key)
			y = v[0]
			sum -= delta

			i++
			if i > rounds-1 {
				break
			}
		}
	}
}

/**
* @Description: bytesToUint32
* @param: in
* @param: inLen
* @param: out
* @param: padding
* @Author: Iori
* @Date: 2022-06-13 20:13:50
**/
func bytesToUint32(in []byte, inLen int, out []uint32, padding bool) {
	// (i & 3) << 3 -> [0, 8, 16, 24]
	for i := 0; i < inLen; i++ {
		out[i>>2] |= uint32(int(in[i]) << ((i & 3) << 3))
	}

	if padding {
		pad := 4 - (inLen & 3)
		if inLen < 4 {
			pad = pad + 4
		}

		for i := inLen; i < inLen+pad; i++ {
			out[i>>2] |= uint32(pad << ((i & 3) << 3))
		}
	}
}

/**
* @Description: uint32sToBytes
* @param: in
* @param: inLen
* @param: out
* @param: padding
* @return: int
* @Author: Iori
* @Date: 2022-06-13 20:13:46
**/
func uint32sToBytes(in []uint32, inLen int, out []byte, padding bool) int {
	for i := 0; i < inLen; i++ {
		out[4*i] = byte(in[i] & 0xFF)
		out[4*i+1] = byte((in[i] >> 8) & 0xFF)
		out[4*i+2] = byte((in[i] >> 16) & 0xFF)
		out[4*i+3] = byte((in[i] >> 24) & 0xFF)
	}

	outLen := inLen * 4
	// PKCS#7 unpadding
	if padding {
		pad := int(out[outLen-1])
		outLen -= pad

		if pad < 1 || pad > 8 {
			return -1
		}

		if outLen < 0 {
			return -2
		}

		for i := outLen; i < inLen*4; i++ {
			if int(out[i]) != pad {
				return -3
			}
		}
	}
	return outLen
}

/**
* @Description: URandom
* @param: n
* @param: seed
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-13 20:13:35
**/
func URandom(n int, seed int64) ([]byte, error) {
	rand.Seed(seed)
	token := make([]byte, n)
	_, err := rand.Read(token)
	if err != nil {
		return nil, err
	}
	return token, nil
}

/**
* @Description: Encrypt
* @param: data
* @param: key
* @param: padding
* @param: rounds
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-13 20:13:18
**/
func Encrypt(data []byte, key []byte, padding bool, rounds uint32) ([]byte, error) {
	var aLen, dLen, kLen, paddingValue int
	dLen, kLen = len(data), len(key)

	if padding {
		paddingValue = 1
	}

	if kLen != 16 {
		return nil, errors.New("need a 16-byte key")
	}

	if !padding && (dLen < 8 || (dLen&3) != 0) {
		return nil, errors.New("data length must be a multiple of 4 bytes and must not be less than 8 bytes")
	}

	if dLen < 4 {
		aLen = 2
	} else {
		aLen = dLen>>2 + paddingValue
	}

	d := make([]uint32, aLen)
	k := make([]uint32, 4)
	bytesToUint32(data, dLen, d, padding)
	bytesToUint32(key, kLen, k, false)
	btea(d, aLen, k, rounds)

	retBuf := make([]byte, aLen<<2)
	_ = uint32sToBytes(d, aLen, retBuf, false)
	return retBuf, nil
}

/**
* @Description: Decrypt
* @param: data
* @param: key
* @param: padding
* @param: rounds
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-13 20:13:09
**/
func Decrypt(data []byte, key []byte, padding bool, rounds uint32) ([]byte, error) {
	var aLen, dLen, kLen, rc int
	dLen, kLen = len(data), len(key)
	if kLen != 16 {
		return nil, errors.New("need a 16-byte key")
	}

	if !padding && (dLen < 8 || (dLen&3) != 0) {
		return nil, errors.New("data length must be a multiple of 4 bytes and must not be less than 8 bytes")
	}

	if (dLen&3) != 0 || dLen < 8 {
		return nil, errors.New("invalid data, data length is not a multiple of 4, or less than 8")
	}

	aLen = dLen / 4
	d := make([]uint32, aLen)
	k := make([]uint32, 4)
	bytesToUint32(data, dLen, d, false)
	bytesToUint32(key, kLen, k, false)
	btea(d, -aLen, k, rounds)

	refBuf := make([]byte, dLen)
	rc = uint32sToBytes(d, aLen, refBuf, padding)

	if padding {
		if rc >= 0 {
			refBuf = refBuf[:rc]
		} else {
			return nil, errors.New("invalid data, illegal PKCS#7 padding. Could be using a wrong key")
		}
	}

	return refBuf, nil
}

/**
* @Description: encrypts data with a key and returns the base64 encoding of the result.
* @param: data
* @param: key
* @param: padding
* @param: rounds
* @return: string
* @return: error
* @Author: Iori
* @Date: 2022-06-13 20:12:43
**/
func EncryptBase64(data []byte, key []byte, padding bool, rounds uint32) (string, error) {
	v, err := Encrypt(data, key, padding, rounds)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(v), nil
}

/**
* @Description: decrypt the base64 encoded data with key.
* @param: b64Str
* @param: key
* @param: padding
* @param: rounds
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-13 20:12:35
**/
func DecryptBase64(b64Str string, key []byte, padding bool, rounds uint32) ([]byte, error) {
	dataBytes, err := base64.StdEncoding.DecodeString(b64Str)
	if err != nil {
		return nil, err
	}

	v, err := Decrypt(dataBytes, key, padding, rounds)
	if err != nil {
		return nil, err
	}
	return v, nil
}

/**
* @Description: encrypts data with a key and returns the hexadecimal encoding of the result.
* @param: data
* @param: key
* @param: padding
* @param: rounds
* @return: string
* @return: error
* @Author: Iori
* @Date: 2022-06-13 20:12:28
**/
func EncryptHex(data []byte, key []byte, padding bool, rounds uint32) (string, error) {
	v, err := Encrypt(data, key, padding, rounds)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(v), nil
}

/**
* @Description: decrypt the hexadecimal encoded data with key.
* @param: hexStr
* @param: key
* @param: padding
* @param: rounds
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-13 20:12:06
**/
func DecryptHex(hexStr string, key []byte, padding bool, rounds uint32) ([]byte, error) {
	dataBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}

	v, err := Decrypt(dataBytes, key, padding, rounds)
	if err != nil {
		return nil, err
	}
	return v, nil
}
