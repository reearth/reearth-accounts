package servicetoken

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// ParsePrivateKeyPEM parses a PEM-encoded RSA private key in PKCS#8 ("PRIVATE
// KEY") or PKCS#1 ("RSA PRIVATE KEY") form, such as the key files make dev-seed
// writes. Its errors never quote the input or a parser error, which can carry
// key material.
func ParsePrivateKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("servicetoken: no PEM block found")
	}
	switch block.Type {
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, errors.New("servicetoken: invalid PKCS#8 private key")
		}
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("servicetoken: PKCS#8 private key is not an RSA key")
		}
		return rk, nil
	case "RSA PRIVATE KEY":
		k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, errors.New("servicetoken: invalid PKCS#1 private key")
		}
		return k, nil
	default:
		return nil, errors.New("servicetoken: PEM block is not an RSA private key")
	}
}
