package service

import (
	"errors"
	"strings"
	"testing"
)

func TestParseOwnOSSObjectURL_VirtualHost(t *testing.T) {
	ep := "oss-cn-beijing.aliyuncs.com"
	bucket := "my-bucket"
	raw := "https://my-bucket.oss-cn-beijing.aliyuncs.com/icons/logo.png?Expires=1&Signature=abc"

	key, ok, err := parseOwnOSSObjectURL(raw, ep, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected own account")
	}
	if key != "icons/logo.png" {
		t.Fatalf("key=%q", key)
	}
}

func TestParseOwnOSSObjectURL_PathStyle(t *testing.T) {
	ep := "oss-cn-beijing.aliyuncs.com"
	bucket := "my-bucket"
	raw := "https://oss-cn-beijing.aliyuncs.com/my-bucket/cover/book%20one.jpg#frag"

	key, ok, err := parseOwnOSSObjectURL(raw, ep, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected own account")
	}
	if key != "cover/book one.jpg" {
		t.Fatalf("key=%q", key)
	}
}

func TestParseOwnOSSObjectURL_WrongHost(t *testing.T) {
	key, ok, err := parseOwnOSSObjectURL(
		"https://cdn.example.com/foo.png",
		"oss-cn-beijing.aliyuncs.com",
		"my-bucket",
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok || key != "" {
		t.Fatalf("ok=%v key=%q", ok, key)
	}
}

func TestParseOwnOSSObjectURL_EmptyPathVirtualHost(t *testing.T) {
	_, ok, err := parseOwnOSSObjectURL(
		"https://my-bucket.oss-cn-beijing.aliyuncs.com/",
		"oss-cn-beijing.aliyuncs.com",
		"my-bucket",
	)
	if ok {
		t.Fatal("expected not ok")
	}
	if err == nil {
		t.Fatal("expected key parse error")
	}
	if !strings.Contains(err.Error(), "无法识别") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseOwnOSSObjectURL_CaseInsensitiveHost(t *testing.T) {
	key, ok, err := parseOwnOSSObjectURL(
		"https://MY-BUCKET.OSS-CN-BEIJING.ALIYUNCS.COM/a/b.png",
		"https://oss-cn-beijing.aliyuncs.com",
		"my-bucket",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || key != "a/b.png" {
		t.Fatalf("ok=%v key=%q", ok, key)
	}
}

func TestParseOwnOSSObjectURL_PathStyleBucketCase(t *testing.T) {
	key, ok, err := parseOwnOSSObjectURL(
		"https://oss-cn-beijing.aliyuncs.com/MY-BUCKET/x.y",
		"oss-cn-beijing.aliyuncs.com",
		"my-bucket",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || key != "x.y" {
		t.Fatalf("ok=%v key=%q", ok, key)
	}
}

func TestParseOwnOSSObjectURL_PathStyleOtherBucketIsExternal(t *testing.T) {
	key, ok, err := parseOwnOSSObjectURL(
		"https://oss-cn-beijing.aliyuncs.com/other-bucket/a.png",
		"oss-cn-beijing.aliyuncs.com",
		"my-bucket",
	)
	if err != nil {
		t.Fatalf("expected external (err=nil), got %v", err)
	}
	if ok || key != "" {
		t.Fatalf("ok=%v key=%q", ok, key)
	}
}

func TestOwnOSS_HeadObjectMeta_NotReady(t *testing.T) {
	s := &OSSService{}
	_, err := s.HeadObjectMeta("k")
	if err == nil || err.Error() != ossNotReadyMsg {
		t.Fatal(err)
	}
}

func TestIsOSSObjectNotFound(t *testing.T) {
	if IsOSSObjectNotFound(nil) {
		t.Fatal("nil")
	}
	wrapped := errors.Join(ErrOSSObjectNotFound, errors.New("detail"))
	if !IsOSSObjectNotFound(wrapped) {
		t.Fatal("expected true")
	}
}