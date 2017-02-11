package ahocorasick

import (
	"fmt"
	"io"
)

type Buffer struct {
	r       io.Reader
	bufSize int

	buf    []byte
	bufEnd int

	readPos int
}

func NewBuffer(r io.Reader, bufSize int) *Buffer {
	return &Buffer{
		r:       r,
		bufSize: bufSize,
		buf:     make([]byte, bufSize*3),
	}
}

func (b *Buffer) ReadByte() (byte, error) {
	var len int
	var err error

	if b.bufEnd == 0 {
		b.bufEnd, err = b.r.Read(b.buf)
		if err != nil {
			return 0, err
		}
	}
	if b.readPos == b.bufSize*2 {
		b.buf = append(append(b.buf[b.bufSize:b.bufSize*2], b.buf[b.bufSize*2:b.bufSize*3]...), b.buf[0:b.bufSize]...)
		b.readPos -= 4

		len, err = b.r.Read(b.buf[b.bufSize*2 : b.bufSize*3])
		b.bufEnd = b.bufEnd - 4 + len
		fmt.Println(b.buf, b.bufEnd)
	}
	if b.readPos == b.bufEnd {
		return 0, io.EOF
	}
	next := b.buf[b.readPos]
	b.readPos++
	return next, nil
}

func (b *Buffer) PeekBack(n int) []byte {
	begin := b.readPos - n - 1
	if begin < 0 {
		begin = 0
	}
	return b.buf[begin : b.readPos-1]
}

func (b *Buffer) PeekForward(n int) []byte {
	end := b.readPos + n
	if end > b.bufEnd {
		end = b.bufEnd
	}
	return b.buf[b.readPos:end]
}
