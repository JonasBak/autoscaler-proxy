package utils

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var runes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}
