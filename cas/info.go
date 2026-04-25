package cas

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Info struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	MD5        string `json:"md5"`
	SliceMD5   string `json:"sliceMd5"`
	CreateTime string `json:"create_time,omitempty"`
}

type payload struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	MD5        string `json:"md5"`
	SliceMD5   string `json:"sliceMd5,omitempty"`
	SliceMD5_  string `json:"slice_md5,omitempty"`
	CreateTime string `json:"create_time,omitempty"`
}

func NewInfo(name string, size int64, md5, sliceMD5 string) *Info {
	return &Info{
		Name:       name,
		Size:       size,
		MD5:        strings.ToLower(md5),
		SliceMD5:   strings.ToLower(sliceMD5),
		CreateTime: strconv.FormatInt(time.Now().Unix(), 10),
	}
}

func (i *Info) Validate() error {
	if i == nil {
		return errors.New("nil cas info")
	}
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("cas source name is empty")
	}
	if i.Size < 0 {
		return errors.New("cas size must be >= 0")
	}
	if !looksLikeMD5(i.MD5) {
		return fmt.Errorf("invalid md5: %q", i.MD5)
	}
	if !looksLikeMD5(i.SliceMD5) {
		return fmt.Errorf("invalid slice_md5: %q", i.SliceMD5)
	}
	return nil
}

func MarshalJSONBytes(info *Info) ([]byte, error) {
	if info == nil {
		return nil, errors.New("nil cas info")
	}
	if err := info.Validate(); err != nil {
		return nil, err
	}
	body := payload{
		Name:       info.Name,
		Size:       info.Size,
		MD5:        strings.ToLower(info.MD5),
		SliceMD5:   strings.ToLower(info.SliceMD5),
		CreateTime: info.CreateTime,
	}
	return json.Marshal(body)
}

func MarshalBase64(info *Info) (string, error) {
	body, err := MarshalJSONBytes(info)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(body), nil
}

func Parse(data []byte) (*Info, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errors.New("empty cas content")
	}
	info, err := parsePayload([]byte(trimmed))
	if err == nil {
		return info, nil
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(trimmed)
	if decodeErr != nil {
		return nil, err
	}
	return parsePayload(decoded)
}

func parsePayload(data []byte) (*Info, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	info := &Info{
		Name:       p.Name,
		Size:       p.Size,
		MD5:        strings.ToLower(p.MD5),
		SliceMD5:   strings.ToLower(firstNonEmpty(p.SliceMD5, p.SliceMD5_)),
		CreateTime: p.CreateTime,
	}
	if err := info.Validate(); err != nil {
		return nil, err
	}
	return info, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func looksLikeMD5(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}
