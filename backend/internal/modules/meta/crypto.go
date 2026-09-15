package meta

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

func (s *Service) seal(plain, purpose string) (string, error) {
	if plain == "" {
		return "", nil
	}
	a, e := s.aeadFor(s.encryptionKeyID())
	if e != nil {
		return "", e
	}
	nonce := make([]byte, a.NonceSize())
	if _, e = io.ReadFull(rand.Reader, nonce); e != nil {
		return "", e
	}
	encoded := base64.RawStdEncoding.EncodeToString(a.Seal(nonce, nonce, []byte(plain), []byte(purpose)))
	if s.encryptionKeyID() != "legacy" {
		encoded = "v2:" + s.encryptionKeyID() + ":" + encoded
	}
	return encoded, nil
}
func (s *Service) open(encoded, purpose string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	keyID := "legacy"
	if strings.HasPrefix(encoded, "v2:") {
		parts := strings.SplitN(encoded, ":", 3)
		if len(parts) != 3 {
			return "", errors.New("加密数据格式无效")
		}
		keyID = parts[1]
		encoded = parts[2]
	}
	a, e := s.aeadFor(keyID)
	if e != nil {
		return "", e
	}
	b, e := base64.RawStdEncoding.DecodeString(encoded)
	if e != nil || len(b) < a.NonceSize() {
		return "", errors.New("Meta 凭证无法解密，请重新保存凭证")
	}
	plain, e := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(purpose))
	if e != nil {
		return "", errors.New("Meta 凭证无法解密，请检查 APP_SECRET 或重新保存凭证")
	}
	return string(plain), nil
}
func (s *Service) encryptionKeyID() string {
	if s.Core.Config.MetaEncryptionKeyID != "" {
		return s.Core.Config.MetaEncryptionKeyID
	}
	return "legacy"
}
func (s *Service) aeadFor(id string) (cipher.AEAD, error) {
	var key []byte
	if id == "legacy" {
		legacy := sha256.Sum256([]byte("linkscope:meta:encryption:v1:" + s.Core.Config.Secret))
		key = legacy[:]
	} else {
		var e error
		key, e = base64.StdEncoding.DecodeString(s.Core.Config.MetaEncryptionKeys[id])
		if e != nil || len(key) != 32 {
			return nil, errors.New("加密密钥不可用，请恢复对应的服务端密钥")
		}
	}
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
