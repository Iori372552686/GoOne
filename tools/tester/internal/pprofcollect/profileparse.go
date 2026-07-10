package pprofcollect

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// countCPUSamples 统计 pprof CPU profile（gzip 压缩的 profile.proto）中的样本条数。
//
// 只做最小化 protobuf wire 扫描：Profile 消息的 field 2（repeated Sample，
// wire type 2）每出现一次即一个样本，其余字段按 wire 规则跳过。
// 每个样本代表 10ms CPU 时间（100Hz 采样）。
func countCPUSamples(gzData []byte) (int64, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		// 某些环境可能返回未压缩数据
		return scanSamples(gzData)
	}
	defer zr.Close()

	raw, err := io.ReadAll(zr)
	if err != nil {
		return 0, err
	}
	return scanSamples(raw)
}

func scanSamples(data []byte) (int64, error) {
	var count int64
	i := 0
	for i < len(data) {
		tag, n := decodeVarint(data[i:])
		if n == 0 {
			return 0, fmt.Errorf("bad varint at %d", i)
		}
		i += n

		fieldNum := tag >> 3
		wireType := tag & 7

		switch wireType {
		case 0: // varint
			_, n := decodeVarint(data[i:])
			if n == 0 {
				return 0, fmt.Errorf("bad varint field at %d", i)
			}
			i += n
		case 1: // fixed64
			i += 8
		case 2: // length-delimited
			length, n := decodeVarint(data[i:])
			if n == 0 {
				return 0, fmt.Errorf("bad length at %d", i)
			}
			i += n + int(length)
			if fieldNum == 2 { // Profile.sample
				count++
			}
		case 5: // fixed32
			i += 4
		default:
			return 0, fmt.Errorf("unsupported wire type %d at %d", wireType, i)
		}
		if i > len(data) {
			return 0, fmt.Errorf("truncated profile")
		}
	}
	return count, nil
}

func decodeVarint(data []byte) (uint64, int) {
	var v uint64
	for i := 0; i < len(data) && i < 10; i++ {
		v |= uint64(data[i]&0x7f) << (7 * i)
		if data[i]&0x80 == 0 {
			return v, i + 1
		}
	}
	return 0, 0
}
