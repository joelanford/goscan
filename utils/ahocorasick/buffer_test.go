package ahocorasick_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/joelanford/goscan/utils/ahocorasick"
	"github.com/stretchr/testify/assert"
)

func TestBuffer(t *testing.T) {
	data := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	buf := ahocorasick.NewBuffer(bytes.NewBuffer(data), 4)

	for i := 0; i < 16; i++ {
		b, err := buf.ReadByte()
		assert.NoError(t, err)
		assert.Equal(t, byte(i), b)
		fmt.Println(buf.PeekBack(2), b, buf.PeekForward(2))
	}

	b, err := buf.ReadByte()
	assert.Equal(t, byte(0), b)
	assert.Equal(t, io.EOF, err)
}

func TestBufferOne(t *testing.T) {
	data := []byte{0}
	buf := ahocorasick.NewBuffer(bytes.NewBuffer(data), 4)

	for i := 0; i < 1; i++ {
		b, err := buf.ReadByte()
		assert.NoError(t, err)
		assert.Equal(t, byte(i), b)
		fmt.Println(buf.PeekBack(2), b, buf.PeekForward(2))
	}

	b, err := buf.ReadByte()
	assert.Equal(t, byte(0), b)
	assert.Equal(t, io.EOF, err)
}
