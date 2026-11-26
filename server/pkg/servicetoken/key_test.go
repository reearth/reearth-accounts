package servicetoken

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePrivateKeyPEM(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	for name, block := range map[string]*pem.Block{
		"pkcs8": {Type: "PRIVATE KEY", Bytes: pkcs8},
		"pkcs1": {Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParsePrivateKeyPEM(pem.EncodeToMemory(block))
			require.NoError(t, err)
			assert.True(t, key.Equal(got))
		})
	}
}

func TestParsePrivateKeyPEM_RefusesOtherInputWithoutQuotingIt(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecDER, err := x509.MarshalPKCS8PrivateKey(ec)
	require.NoError(t, err)
	garbage := []byte("not-a-key-0123456789abcdef")

	for name, in := range map[string][]byte{
		"not PEM":         garbage,
		"ecdsa PKCS#8":    pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecDER}),
		"bad PKCS#8":      pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: garbage}),
		"bad PKCS#1":      pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: garbage}),
		"public key type": pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: garbage}),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParsePrivateKeyPEM(in)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), string(garbage))
			assert.NotContains(t, err.Error(), "-----BEGIN")
		})
	}
}
