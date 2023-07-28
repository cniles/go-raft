package util

import (
	"math/rand"
	"time"
)

func RandomTimeout(min, max int64) time.Duration {
	d := max - min
	rnd := rand.Int63n(d)
	timeout := rnd + min
	return time.Duration(timeout) * time.Millisecond
}
