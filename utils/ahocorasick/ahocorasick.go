package ahocorasick

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/joelanford/goscan/utils/darts"
)

const FAIL_STATE = -1
const ROOT_STATE = 1

type Machine struct {
	trie    *darts.DoubleArrayTrie
	failure []int
	output  map[int]([][]byte)
}

type Term struct {
	Pos     int
	Word    []byte
	Context []byte
}

func (m *Machine) Build(keywords [][]byte) (err error) {
	if len(keywords) == 0 {
		return errors.New("empty keywords")
	}

	d := new(darts.Darts)

	trie := new(darts.LinkedListTrie)
	m.trie, trie, err = d.Build(keywords)
	if err != nil {
		return err
	}

	m.output = make(map[int]([][]byte), 0)
	for idx, val := range d.Output {
		m.output[idx] = append(m.output[idx], val)
	}

	queue := make([](*darts.LinkedListTrieNode), 0)
	m.failure = make([]int, len(m.trie.Base))
	for _, c := range trie.Root.Children {
		m.failure[c.Base] = darts.ROOT_NODE_BASE
	}
	queue = append(queue, trie.Root.Children...)

	for {
		if len(queue) == 0 {
			break
		}

		node := queue[0]
		for _, n := range node.Children {
			if n.Base == darts.END_NODE_BASE {
				continue
			}
			inState := m.f(node.Base)
		set_state:
			outState := m.g(inState, n.Code-darts.ROOT_NODE_BASE)
			if outState == FAIL_STATE {
				inState = m.f(inState)
				goto set_state
			}
			if _, ok := m.output[outState]; ok != false {
				m.output[n.Base] = append(m.output[outState], m.output[n.Base]...)
			}
			m.setF(n.Base, outState)
		}
		queue = append(queue, node.Children...)
		queue = queue[1:]
	}

	return nil
}

func (m *Machine) g(inState int, input byte) (outState int) {
	if inState == FAIL_STATE {
		return ROOT_STATE
	}

	t := inState + int(input) + darts.ROOT_NODE_BASE
	if t >= len(m.trie.Base) {
		if inState == ROOT_STATE {
			return ROOT_STATE
		}
		return FAIL_STATE
	}
	if inState == m.trie.Check[t] {
		return m.trie.Base[t]
	}

	if inState == ROOT_STATE {
		return ROOT_STATE
	}

	return FAIL_STATE
}

func (m *Machine) f(index int) (state int) {
	return m.failure[index]
}

func (m *Machine) setF(inState, outState int) {
	m.failure[inState] = outState
}

func (m *Machine) MultiPatternSearch(content []byte, context int, returnImmediately bool) [](*Term) {
	terms := make([](*Term), 0)

	state := ROOT_STATE
	for pos, c := range content {
	start:
		if m.g(state, c) == FAIL_STATE {
			state = m.f(state)
			goto start
		} else {
			state = m.g(state, c)
			if val, ok := m.output[state]; ok != false {
				for _, word := range val {
					term := new(Term)
					term.Pos = pos - len(word) + 1
					term.Word = word

					contextBegin := term.Pos - context
					contextEnd := term.Pos + len(word) + context
					if contextBegin < 0 {
						contextBegin = 0
					}
					if contextEnd > len(content) {
						contextEnd = len(content)
					}
					term.Context = content[contextBegin:contextEnd]

					terms = append(terms, term)
					if returnImmediately {
						return terms
					}
				}
			}
		}
	}

	return terms
}

func (m *Machine) MultiPatternSearchReader(r io.Reader, context int, returnImmediately bool) ([]*Term, error) {
	bufSize := 4096
	if context > bufSize/2 {
		return nil, errors.New("context cannot exceed " + strconv.Itoa(bufSize/2) + " bytes")
	}
	terms := make([](*Term), 0)

	var err error
	var prevEnd int
	var currEnd int
	var nextEnd int

	var prevBuf []byte
	currBuf := make([]byte, bufSize)
	nextBuf := make([]byte, bufSize)

	currEnd, err = r.Read(currBuf)
	if err != nil {
		if err == io.EOF {
			return terms, nil
		}
		return nil, err
	}
	totalPos := 0

	state := ROOT_STATE
	for currBuf != nil {
		nextEnd, err = r.Read(nextBuf)
		if err != nil {
			if err == io.EOF {
				nextBuf = nil
				nextEnd = 0
			} else {
				return nil, err
			}
		}

		for pos, c := range currBuf {
		start:
			if m.g(state, c) == FAIL_STATE {
				state = m.f(state)
				goto start
			} else {
				state = m.g(state, c)
				if val, ok := m.output[state]; ok != false {
					for _, word := range val {
						term := new(Term)
						wordBegin := pos - len(word) + 1
						wordEnd := wordBegin + len(word)
						term.Pos = totalPos - len(word) + 1
						term.Word = word

						var contextBefore []byte
						contextBegin := wordBegin - context
						if contextBegin < 0 {
							if prevBuf == nil {
								contextBefore = currBuf[0:wordBegin]
							} else if wordBegin < 0 {
								contextBefore = prevBuf[prevEnd+contextBegin : prevEnd+wordBegin]
							} else {
								contextBefore = append(prevBuf[prevEnd+contextBegin:bufSize], currBuf[0:wordBegin]...)
							}
						} else {
							contextBefore = currBuf[contextBegin:wordBegin]
						}

						var contextAfter []byte
						contextEnd := wordEnd + context
						if contextEnd > currEnd {
							if nextBuf == nil {
								fmt.Println(totalPos, pos, wordEnd, currEnd, len(currBuf))
								contextAfter = currBuf[wordEnd:currEnd]
							} else if contextEnd-currEnd > nextEnd {
								contextAfter = append(currBuf[wordEnd:currEnd], nextBuf[0:nextEnd]...)
							} else {
								contextAfter = append(currBuf[wordEnd:currEnd], nextBuf[0:contextEnd-currEnd]...)
							}
						} else {
							contextAfter = currBuf[wordEnd:contextEnd]
						}
						context := append(append(contextBefore, word...), contextAfter...)
						term.Context = make([]byte, len(context))
						copy(term.Context, context)

						terms = append(terms, term)
						if returnImmediately {
							return terms, nil
						}
					}
				}
			}
			totalPos++
		}

		prevBuf = currBuf[0:currEnd]
		prevEnd = currEnd
		currBuf = nextBuf[0:nextEnd]
		currEnd = nextEnd
		nextBuf = prevBuf[0:4096]

	}
	return terms, nil
}

func (m *Machine) ExactSearch(content []byte) [](*Term) {
	if m.trie.ExactMatchSearch(content, 0) {
		t := new(Term)
		t.Word = content
		t.Pos = 0
		return [](*Term){t}
	}

	return nil
}
