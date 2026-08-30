package utils

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

func ParseCertificateFromBase64(clientCertificateData string) (*x509.Certificate, error) {
	// 解码 base64
	certBytes, err := base64.StdEncoding.DecodeString(clientCertificateData)
	if err != nil {
		panic(fmt.Sprintf("base64 解码失败: %v", err))
	}

	// PEM 解码
	block, _ := pem.Decode(certBytes)
	if block == nil {
		panic("PEM 解码失败")
	}

	// 解析 X.509 证书
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic(fmt.Sprintf("证书解析失败: %v", err))
	}

	return cert, nil
}
