package joiner

import (
	"os"
	"sync"
)

type MemoryJoiner struct {
	l      sync.Mutex
	blocks map[int][]byte
	file   *os.File
	index  int
}

func NewMem(outFile string) (*MemoryJoiner, error) {
	f, err := os.OpenFile(outFile, os.O_CREATE|os.O_TRUNC|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	joiner := &MemoryJoiner{
		blocks: map[int][]byte{},
		file:   f,
	}

	return joiner, nil
}

func (j *MemoryJoiner) Add(id int, block []byte) error {
	j.l.Lock()
	j.blocks[id] = block
	err := j.merge()
	j.l.Unlock()
	return err
}

func (j *MemoryJoiner) merge() error {
	for {
		block, ok := j.blocks[j.index]
		if ok {
			_, err := j.file.Write(block)
			if err != nil {
				return err
			}
			delete(j.blocks, j.index)
			j.index++
		} else {
			break
		}
	}
	return nil
}

func (j *MemoryJoiner) Merge() error {
	return j.file.Close()
}
