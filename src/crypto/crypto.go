package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	env "ovk-im/src/config"
)

func Encrypt(plaintext string) (string, error) {
	secretKey := env.Get("SECRET_KEY", "error")
	hasher := sha256.New()
	hasher.Write([]byte(secretKey))
	key := hasher.Sum(nil)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

func Decrypt(encryptedHex string) (string, error) {
	secretKey := env.Get("SECRET_KEY", "error")
	data, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write([]byte(secretKey))
	key := hasher.Sum(nil)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("invalid encrypted data")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
