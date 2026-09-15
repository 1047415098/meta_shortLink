package requestlogs

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// URL token 的密钥与 Meta 凭证分开派生；APP_SECRET 需要随数据库备份保留。
func (a *Handler) queryTokenCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("linkscope:request-log:query-token:v1:" + a.Config.Secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// 仅保存名为 token 的 URL 参数，保留重复参数和空值，其余敏感字段沿用脱敏规则。
func (a *Handler) sealQueryToken(id string, values url.Values) (string, error) {
	tokens := map[string][]string{}
	for key, values := range values {
		if strings.EqualFold(key, "token") {
			tokens[key] = values
		}
	}
	if len(tokens) == 0 {
		return "", nil
	}
	plain, err := json.Marshal(tokens)
	if err != nil {
		return "", err
	}
	// 与日志正文保持同一大小上限；超限时仍保存原有脱敏日志。
	if len(plain) > logBodyLimit {
		return "", nil
	}
	aead, err := a.queryTokenCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	// 将密文绑定到日志编号，防止不同日志之间交换密文后显示错误凭证。
	sealed := aead.Seal(nonce, nonce, plain, []byte("request-log-query-token:"+id))
	return "v1:" + base64.RawStdEncoding.EncodeToString(sealed), nil
}

// 解密失败只返回固定错误，避免将输入值或密文带入应用日志。
func (a *Handler) openQueryToken(id, encoded string) (map[string][]string, error) {
	invalid := errors.New("request log token cannot be decrypted")
	if !strings.HasPrefix(encoded, "v1:") {
		return nil, invalid
	}
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(encoded, "v1:"))
	if err != nil {
		return nil, invalid
	}
	aead, err := a.queryTokenCipher()
	if err != nil || len(data) < aead.NonceSize()+aead.Overhead() {
		return nil, invalid
	}
	plain, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte("request-log-query-token:"+id))
	if err != nil {
		return nil, invalid
	}
	var tokens map[string][]string
	if json.Unmarshal(plain, &tokens) != nil {
		return nil, invalid
	}
	for key := range tokens {
		if !strings.EqualFold(key, "token") {
			return nil, invalid
		}
	}
	return tokens, nil
}
