package panel

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const tokenSecretFile = "token.key"

// tokenCipher 用本机密钥加密 Authorization 明文，便于管理员随时查看。
// 密钥保存在数据目录，与数据库分离；密钥丢失时只能重新生成令牌。
type tokenCipher struct {
	key []byte
}

func loadTokenCipher(root string) (*tokenCipher, error) {
	path := filepath.Join(root, tokenSecretFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
			return nil, err
		}
		return &tokenCipher{key: key}, nil
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("token 密钥文件损坏，请删除 data/token.key 后重新生成令牌")
	}
	return &tokenCipher{key: key}, nil
}

func (cipherSet *tokenCipher) encrypt(plain string) (string, error) {
	if cipherSet == nil || len(cipherSet.key) != 32 {
		return "", errors.New("token 密钥不可用")
	}
	block, err := aes.NewCipher(cipherSet.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (cipherSet *tokenCipher) decrypt(encoded string) (string, error) {
	if cipherSet == nil || len(cipherSet.key) != 32 {
		return "", errors.New("token 密钥不可用")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(cipherSet.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("token 密文损坏")
	}
	nonce, data := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
