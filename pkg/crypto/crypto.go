package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// prefix marks an encrypted value so we can distinguish from legacy plaintext.
	Prefix     = "enc:"
	saltLen    = 16
	keyLen     = 32 // AES-256
	iterations = 100_000
)

// DeriveKey produces a 32-byte AES key from machine-unique material.
// The key never leaves memory unencrypted; callers must call ZeroBytes when done.
func DeriveKey(salt []byte) []byte {
	material := machineMaterial()
	return pbkdf2.Key(material, salt, iterations, keyLen, sha256.New)
}

// Encrypt plaintext using AES-256-GCM with a random nonce and salt.
// Returns a base64 string with the Prefix tag: "enc:<base64(salt|nonce|ciphertext+tag)>"
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("crypto: generate salt: %w", err)
	}

	key := DeriveKey(salt)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new GCM: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)

	// pack: salt || nonce || ciphertext(+tag)
	blob := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	blob = append(blob, salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	return Prefix + base64.StdEncoding.EncodeToString(blob), nil
}

// Decrypt a value produced by Encrypt. If the value lacks the Prefix tag
// it is returned as-is (plaintext migration).
func Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	if !strings.HasPrefix(encoded, Prefix) {
		// Legacy plaintext value — return unchanged.
		return encoded, nil
	}

	raw, err := base64.StdEncoding.DecodeString(encoded[len(Prefix):])
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}

	if len(raw) < saltLen {
		return "", errors.New("crypto: blob too short")
	}

	salt := raw[:saltLen]
	rest := raw[saltLen:]

	key := DeriveKey(salt)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new GCM: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(rest) < nonceSize {
		return "", errors.New("crypto: nonce missing")
	}

	nonce := rest[:nonceSize]
	ciphertext := rest[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt failed: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted returns true if the value carries the encryption prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, Prefix)
}

// ZeroBytes overwrites a byte slice with zeros to scrub key material from memory.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ZeroString is a best-effort scrub for strings. Go strings are immutable so
// this only helps if the caller drops all references immediately after.
func ZeroString(s *string) {
	*s = ""
}

// machineMaterial returns a deterministic secret derived from the current
// machine identity. This is not meant to be portable — it ties encrypted
// values to the machine and user that created them.
func machineMaterial() []byte {
	hostname, _ := os.Hostname()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	osName := runtime.GOOS

	h := sha256.New()
	h.Write([]byte(hostname))
	h.Write([]byte{0})
	h.Write([]byte(username))
	h.Write([]byte{0})
	h.Write([]byte(osName))
	h.Write([]byte{0})
	// Application-specific pepper so cli_mate keys don't collide with other apps.
	h.Write([]byte("cli_mate.salt.v1"))
	return h.Sum(nil)
}
