package tiktok

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"whatsapp-analytics/internal/platform/runtime"
)

func (s *Service) seal(plain, purpose string) (string, error) {
	if plain == "" {
		return "", nil
	}
	aead, err := s.aeadFor(s.encryptionKeyID())
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	encoded := base64.RawStdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(plain), []byte(purpose)))
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
			return "", errors.New("TikTok 加密数据格式无效")
		}
		keyID, encoded = parts[1], parts[2]
	}
	aead, err := s.aeadFor(keyID)
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < aead.NonceSize() {
		return "", errors.New("TikTok 凭证无法解密，请重新保存凭证")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(purpose))
	if err != nil {
		return "", errors.New("TikTok 凭证无法解密，请检查服务端密钥或重新保存凭证")
	}
	return string(plain), nil
}

func (s *Service) encryptionKeyID() string {
	if s.Core.Config.MetaEncryptionKeyID != "" {
		return s.Core.Config.MetaEncryptionKeyID
	}
	return "legacy"
}

func (s *Service) aeadFor(keyID string) (cipher.AEAD, error) {
	var key []byte
	if keyID == "legacy" {
		legacy := sha256.Sum256([]byte("linkscope:tiktok:encryption:v1:" + s.Core.Config.Secret))
		key = legacy[:]
	} else {
		var err error
		key, err = base64.StdEncoding.DecodeString(s.Core.Config.MetaEncryptionKeys[keyID])
		if err != nil || len(key) != 32 {
			return nil, errors.New("TikTok 加密密钥不可用，请恢复对应的服务端密钥")
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Service) sealToken(token string, connectionID int64) (string, error) {
	return s.seal(token, fmt.Sprintf("tiktok:token:%d", connectionID))
}

func (s *Service) sealEvent(payload string, eventID string) (string, error) {
	return s.seal(payload, "tiktok:event:"+eventID)
}

// SealVisitContext protects the complete landing context with a visit-specific
// purpose so it cannot be opened as a token, event payload or another visit.
func (s *Service) SealVisitContext(input VisitContext, visitID string) (string, error) {
	input.IP = runtime.Bounded(strings.TrimSpace(input.IP), 64)
	input.UserAgent = runtime.Bounded(input.UserAgent, 1024)
	input.PageURL = runtime.Bounded(input.PageURL, 4096)
	input.Referrer = runtime.Bounded(input.Referrer, 2048)
	payload, err := json.Marshal(input)
	if err != nil {
		return "", errors.New("TikTok 访问上下文无法编码")
	}
	return s.seal(string(payload), "tiktok:visit:"+visitID)
}
