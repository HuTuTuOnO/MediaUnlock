package utils

// 通用工具:安全随机串生成。
//   RandomHex     —— 十六进制串,用于节点 token 等
//   RandomURLSafe —— URL-safe base64 串,用于初始密码等

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// RandomHex 返回 nBytes 字节的加密随机数据的十六进制串(长度 = 2*nBytes)。
func RandomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// RandomURLSafe 返回 nBytes 字节的加密随机数据的 URL-safe base64 串(无填充)。
func RandomURLSafe(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
