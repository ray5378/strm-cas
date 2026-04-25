package cas

import "testing"

func TestParseMarshalBase64(t *testing.T) {
	info := &Info{
		Name:       "movie.mkv",
		Size:       123,
		MD5:        "d41d8cd98f00b204e9800998ecf8427e",
		SliceMD5:   "d41d8cd98f00b204e9800998ecf8427e",
		CreateTime: "1740000000",
	}
	body, err := MarshalBase64(info)
	if err != nil {
		t.Fatalf("MarshalBase64 err: %v", err)
	}
	parsed, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse err: %v", err)
	}
	if parsed.Name != info.Name || parsed.Size != info.Size || parsed.MD5 != info.MD5 || parsed.SliceMD5 != info.SliceMD5 {
		t.Fatalf("parsed mismatch: %#v", parsed)
	}
}
