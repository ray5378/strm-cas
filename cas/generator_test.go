package cas

import (
	"bytes"
	"testing"
)

func TestGenerateSingleChunk(t *testing.T) {
	data := []byte("hello world")
	info, err := Generate(bytes.NewReader(data), "hello.txt", int64(len(data)), 1024)
	if err != nil {
		t.Fatalf("Generate err: %v", err)
	}
	if info.Name != "hello.txt" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.MD5 != "5eb63bbbe01eeed093cb22bb8f5acdc3" {
		t.Fatalf("unexpected md5: %s", info.MD5)
	}
	if info.SliceMD5 != info.MD5 {
		t.Fatalf("single chunk slice md5 should equal file md5: %s vs %s", info.SliceMD5, info.MD5)
	}
}

func TestGenerateMultiChunk(t *testing.T) {
	data := []byte("abcdefghijklmnop")
	info, err := Generate(bytes.NewReader(data), "x.bin", int64(len(data)), 4)
	if err != nil {
		t.Fatalf("Generate err: %v", err)
	}
	if info.MD5 == "" || info.SliceMD5 == "" {
		t.Fatalf("empty hashes: %#v", info)
	}
	if info.MD5 == info.SliceMD5 {
		t.Fatalf("multi-chunk slice md5 should differ from file md5 in this case")
	}
}

func TestPartSize189PC(t *testing.T) {
	if got := ChunkSize(Mode189PC, 1); got != 10*1024*1024 {
		t.Fatalf("unexpected small chunk size: %d", got)
	}
	mid := int64(10*1024*1024*999 + 1)
	if got := ChunkSize(Mode189PC, mid); got != 20*1024*1024 {
		t.Fatalf("unexpected mid chunk size: %d", got)
	}
}
