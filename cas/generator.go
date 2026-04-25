package cas

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func GenerateFromPath(path string, mode Mode) (*Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open source file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat source file: %w", err)
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("source is a directory: %s", path)
	}

	name := filepath.Base(path)
	size := stat.Size()
	chunkSize := ChunkSize(mode, size)
	return Generate(f, name, size, chunkSize)
}

func Generate(r io.ReadSeeker, name string, size int64, chunkSize int64) (*Info, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("empty source name")
	}
	if size < 0 {
		return nil, fmt.Errorf("invalid size")
	}
	if chunkSize <= 0 {
		chunkSize = size
	}
	if chunkSize <= 0 {
		chunkSize = 1
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek source start: %w", err)
	}

	fullHasher := md5.New()
	partMD5s := make([]string, 0)
	remaining := size
	bufSize := chunkSize
	if bufSize > 1024*1024 {
		bufSize = 1024 * 1024
	}
	if bufSize <= 0 {
		bufSize = 1
	}
	buf := make([]byte, bufSize)

	for remaining > 0 {
		partRemaining := minInt64(chunkSize, remaining)
		partHasher := md5.New()
		for partRemaining > 0 {
			readSize := int(minInt64(int64(len(buf)), partRemaining))
			n, readErr := io.ReadFull(r, buf[:readSize])
			if n > 0 {
				fullHasher.Write(buf[:n])
				partHasher.Write(buf[:n])
				partRemaining -= int64(n)
				remaining -= int64(n)
			}
			if readErr != nil {
				if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
					break
				}
				return nil, fmt.Errorf("read source: %w", readErr)
			}
		}
		partMD5s = append(partMD5s, strings.ToUpper(hex.EncodeToString(partHasher.Sum(nil))))
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek reset: %w", err)
	}

	fileMD5 := hex.EncodeToString(fullHasher.Sum(nil))
	sliceMD5 := fileMD5
	if len(partMD5s) > 1 {
		h := md5.Sum([]byte(strings.Join(partMD5s, "\n")))
		sliceMD5 = hex.EncodeToString(h[:])
	}
	return NewInfo(name, size, fileMD5, sliceMD5), nil
}

func WriteCASFile(dstPath string, info *Info) error {
	body, err := MarshalBase64(info)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, []byte(body), 0o644)
}

func GenerateAndWrite(srcPath, dstPath string, mode Mode) (*Info, error) {
	info, err := GenerateFromPath(srcPath, mode)
	if err != nil {
		return nil, err
	}
	if err := WriteCASFile(dstPath, info); err != nil {
		return nil, err
	}
	return info, nil
}

func DefaultOutputPath(srcPath string) string {
	return srcPath + ".cas"
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
