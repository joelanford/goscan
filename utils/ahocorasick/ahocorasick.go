package ahocorasick

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/joelanford/goscan/utils/darts"
)

const FAIL_STATE = -1
const ROOT_STATE = 1

type Machine struct {
	trie       *darts.DoubleArrayTrie
	failure    []int
	output     map[int]([][]byte)
	longestLen int
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

	for _, k := range keywords {
		if len(k) > m.longestLen {
			m.longestLen = len(k)
			if m.longestLen > 4096 {
				return errors.New("keywords longer than 4096 bytes not supported")
			}
		}
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

func (m *Machine) MultiPatternSearchReadSeeker(f io.ReadSeeker, context int, returnImmediately bool) ([]*Term, error) {
	bufSize := 4096
	maxContext := bufSize - m.longestLen + 1
	if context > maxContext {
		return nil, errors.New("context cannot exceed " + strconv.Itoa(maxContext) + " bytes")
	}

	errChan := make(chan error)
	bufChan := make(chan []byte)
	termsChan := make(chan *Term)

	go func() {
		defer close(bufChan)
		for i := int64(0); true; i += 1036288 {
			_, err := f.Seek(i, 0)
			if err != nil {
				errChan <- err
				return
			}
			buf := make([]byte, 1048576)
			len, err := f.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				errChan <- err
				return
			}
			bufChan <- buf[0:len]
		}
	}()

	var searchWg sync.WaitGroup
	searchWg.Add(4)
	for i := 0; i < 4; i++ {
		go func() {
			defer searchWg.Done()
			for buf := range bufChan {
				for _, term := range m.MultiPatternSearch(buf, context, returnImmediately) {
					termsChan <- term
				}
			}
		}()
	}

	go func() {
		searchWg.Wait()
		close(termsChan)
	}()

	termsMap := make(map[string]bool)
	terms := make([]*Term, 0)
	for {
		select {
		case err := <-errChan:
			return terms, err
		case term, ok := <-termsChan:
			if !ok {
				return terms, nil
			}
			key := fmt.Sprintf("%d:%s", term.Pos, string(term.Word))
			if _, ok := termsMap[key]; !ok {
				termsMap[key] = true
				terms = append(terms, term)
			}
		}
	}
}

func (m *Machine) MultiPatternSearchReader(r io.Reader, context int, returnImmediately bool) ([]*Term, error) {
	bufSize := 4096
	maxContext := bufSize - m.longestLen + 1
	if context > maxContext {
		return nil, errors.New("context cannot exceed " + strconv.Itoa(maxContext) + " bytes")
	}
	terms := make([](*Term), 0)

	var err error
	var prevEnd int
	var currEnd int
	var nextEnd int

	prevBuf := make([]byte, bufSize)
	currBuf := make([]byte, bufSize)
	nextBuf := make([]byte, bufSize)
	tempBuf := make([]byte, bufSize)

	currEnd, err = r.Read(currBuf)
	if err != nil {
		if err == io.EOF {
			return terms, nil
		}
		return nil, err
	}
	totalPos := 0

	state := ROOT_STATE
	for currEnd != 0 {
		nextEnd, err = r.Read(nextBuf)
		if err != nil {
			if err == io.EOF {
				nextEnd = 0
			} else {
				return nil, err
			}
		}

		for pos, c := range currBuf[0:currEnd] {
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
							if prevEnd == 0 {
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
							if nextEnd == 0 {
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

		tempBuf = prevBuf
		prevBuf = currBuf[0:currEnd]
		prevEnd = currEnd
		currBuf = nextBuf[0:nextEnd]
		currEnd = nextEnd
		nextBuf = tempBuf[0:4096]
	}
	return terms, nil
}
