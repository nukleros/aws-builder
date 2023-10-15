package util

import (
	"math/rand"
	"time"
)

const alphaNumericCharset = "abcdefghijklmnopqrstuvwxyz0123456789"

// RandomAlphaNumericString returns a random string with the provided length
// using alpha-numeric charcaters.
func RandomAlphaNumericString(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	bytes := make([]byte, length)
	for i := range bytes {
		bytes[i] = alphaNumericCharset[seededRand.Intn(len(alphaNumericCharset))]
	}

	return string(bytes)
}
