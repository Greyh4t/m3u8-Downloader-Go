package joiner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type FFmepgJoiner struct {
	l        sync.Mutex
	ffmpeg   string
	outFile  string
	cacheDir string
	blocks   map[int]string
}

func NewFFmepg(ffmpeg string, outFile string) (*FFmepgJoiner, error) {
	joiner := &FFmepgJoiner{
		ffmpeg:  ffmpeg,
		outFile: outFile,
		blocks:  map[int]string{},
	}

	_, err := exec.LookPath(ffmpeg)
	if err != nil {
		return nil, err
	}

	dir, err := joiner.mkdir()
	if err != nil {
		return nil, err
	}
	joiner.cacheDir = dir

	return joiner, nil
}

func (j *FFmepgJoiner) mkdir() (string, error) {
	cache, err := os.MkdirTemp("./", "m3u8_cache_*")
	if err != nil {
		return "", err
	}
	return filepath.Abs(cache)
}

func (j *FFmepgJoiner) Add(id int, block []byte) error {
	file := filepath.Join(j.cacheDir, fmt.Sprintf("%d.ts", id))
	err := os.WriteFile(file, block, 0644)
	if err != nil {
		return err
	}

	j.l.Lock()
	j.blocks[id] = file
	j.l.Unlock()
	return err
}

func (j *FFmepgJoiner) merge(mergeFile string) error {
	cmd := exec.Command(j.ffmpeg, "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", mergeFile, "-c", "copy", j.outFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (j *FFmepgJoiner) Merge() error {
	var text string
	i := 0
	for {
		file, ok := j.blocks[i]
		if !ok {
			break
		}
		text += fmt.Sprintf("file '%s'\n", file)
		i++
	}

	mergeFile := filepath.Join(j.cacheDir, "merge_list.txt")
	err := os.WriteFile(mergeFile, []byte(text), 0644)
	if err != nil {
		return err
	}

	err = j.merge(mergeFile)
	if err != nil {
		return fmt.Errorf("ffmpeg merge error: %w", err)
	}

	if j.cacheDir != "" {
		os.RemoveAll(j.cacheDir)
	}

	return nil
}
