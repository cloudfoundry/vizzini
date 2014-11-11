package acceptance_test

import (
	"io"
	"math/rand"
	"time"
)

//gratuitously snagged from https://github.com/dustin/randbo/blob/master/randbo.go

type randbo struct {
	rand.Source
}

func NewRandbo() io.Reader {
	return &randbo{rand.NewSource(time.Now().UnixNano())}
}

func (r *randbo) Read(p []byte) (n int, err error) {
	todo := len(p)
	offset := 0
	for {
		val := int64(r.Int63())
		for i := 0; i < 8; i++ {
			p[offset] = byte(val)
			todo--
			if todo == 0 {
				return len(p), nil
			}
			offset++
			val >>= 8
		}
	}
}
