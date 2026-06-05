package utils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// StringHash computes the SHA-256 hash of a string.
func StringHash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

func SymbolSignatureHash(symbol, path, signature string) string {
	return StringHash(fmt.Sprintf("%s:%s:%s", symbol, path, signature))
}

// CalculateHash computes the SHA-256 hash of a file.
// It uses io.Copy to stream the file into the hasher to minimize memory usage.
func CalculateHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// HashString computes the SHA-256 hash of a string.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:])
}

// Shadow Indexing Test
