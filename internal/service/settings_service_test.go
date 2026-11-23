package service

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"assethub/internal/repo"
)

func TestMergeKeepsSecretWhenEmpty(t *testing.T) {
	old := repo.SettingsRow{AccessKeySecret: "oldsecret", BusinessToken: "tok", AccessKeyID: "LTAIOLDKEY"}
	merged := mergeSettings(old, SettingsUpdateReq{
		Endpoint:           "oss-cn-hangzhou.aliyuncs.com",
		Bucket:             "bucket",
		AccessKeyID:        "",
		AccessKeySecret:    "",
		URLPrefix:          "https://bucket.oss-cn-hangzhou.aliyuncs.com",
		BusinessToken:      "",
		Private:            true,
		SignedExpire:       3600,
		ClearBusinessToken: false,
	})
	if merged.AccessKeySecret != "oldsecret" || merged.BusinessToken != "tok" || merged.AccessKeyID != "LTAIOLDKEY" {
		t.Fatal(merged)
	}
	if merged.Endpoint != "oss-cn-hangzhou.aliyuncs.com" || merged.Bucket != "bucket" || !merged.Private || merged.SignedExpire != 3600 {
		t.Fatal(merged)
	}
	merged2 := mergeSettings(old, SettingsUpdateReq{ClearBusinessToken: true})
	if merged2.BusinessToken != "" {
		t.Fatal(merged2)
	}
	if merged2.AccessKeySecret != "oldsecret" {
		t.Fatal(merged2)
	}
}

func TestMergeReplacesSecretsAndClearsToken(t *testing.T) {
	old := repo.SettingsRow{
		Endpoint:        "old-ep",
		Bucket:          "old-b",
		AccessKeyID:     "LTAIOLDKEY",
		AccessKeySecret: "oldsecret",
		URLPrefix:       "https://old.example",
		BusinessToken:   "tok",
		Private:         false,
		SignedExpire:    60,
	}
	merged := mergeSettings(old, SettingsUpdateReq{
		Endpoint:           "new-ep",
		Bucket:             "new-b",
		AccessKeyID:        "LTAINEWKEY",
		AccessKeySecret:    "newsecret",
		URLPrefix:          "https://new.example",
		BusinessToken:      "ignored",
		Private:            true,
		SignedExpire:       120,
		ClearBusinessToken: true,
	})
	if merged.AccessKeyID != "LTAINEWKEY" || merged.AccessKeySecret != "newsecret" {
		t.Fatal(merged)
	}
	if merged.BusinessToken != "" {
		t.Fatalf("clear should ignore body token: %+v", merged)
	}
	if merged.Endpoint != "new-ep" || merged.Bucket != "new-b" || merged.URLPrefix != "https://new.example" || !merged.Private || merged.SignedExpire != 120 {
		t.Fatal(merged)
	}
}

func TestMergePlainEmptyOverwrites(t *testing.T) {
	old := repo.SettingsRow{Endpoint: "keep-me", Bucket: "b", URLPrefix: "https://x", SignedExpire: 10, Private: true}
	merged := mergeSettings(old, SettingsUpdateReq{})
	if merged.Endpoint != "" || merged.Bucket != "" || merged.URLPrefix != "" || merged.SignedExpire != 0 || merged.Private {
		t.Fatalf("plain fields must follow request zeros: %+v", merged)
	}
}

func TestMaskAK(t *testing.T) {
	if maskAK("") != "" {
		t.Fatal(maskAK(""))
	}
	if maskAK("short") != "****" {
		t.Fatal(maskAK("short"))
	}
	if got := maskAK("LTAI1234abcd"); got != "LTAI****abcd" {
		t.Fatal(got)
	}
	if maskAK("12345678") != "****" {
		t.Fatal("len 8")
	}
}

func TestGenObjectKeyPrefix(t *testing.T) {
	s := &OSSService{}
	if got := s.GenObjectKey("icon", "logo", "png"); got != "icon/logo.png" {
		t.Fatal(got)
	}
	if got := s.GenObjectKey("cover/", "book", "jpg"); got != "cover/book.jpg" {
		t.Fatal(got)
	}
}

func TestOSSNotReadyGuards(t *testing.T) {
	s := &OSSService{}
	if s.Ready() {
		t.Fatal("expected not ready")
	}
	err := s.PutObject("k", bytes.NewReader(nil), 0, "image/png")
	if err == nil || err.Error() != ossNotReadyMsg {
		t.Fatal(err)
	}
	if _, err := s.URL("k"); err == nil || err.Error() != ossNotReadyMsg {
		t.Fatal(err)
	}
	if err := s.DeleteObject("k"); err == nil || err.Error() != ossNotReadyMsg {
		t.Fatal(err)
	}
	if err := s.DeleteObject(""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.URL(""); err != nil {
		t.Fatal(err)
	}
}

type stubCounter struct {
	n   int
	err error
}

func (s stubCounter) Count() (int, error) { return s.n, s.err }

func TestViewCategoryCount(t *testing.T) {
	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := repo.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := NewSettingsService(repo.NewSettingsRepo(db), nil)
	v := s.View()
	if v.CategoryCount != 0 || v.OssReady || v.SetupComplete || v.OssError == "" {
		t.Fatalf("%+v", v)
	}
	if s.BusinessToken() != "" {
		t.Fatal(s.BusinessToken())
	}
	if s.OSS() == nil || s.OSS().Ready() {
		t.Fatal("expected unready shell")
	}
	st := s.SetupStatus()
	if st.CategoryCount != 0 || st.OssReady || st.SetupComplete || st.OssError == "" {
		t.Fatalf("%+v", st)
	}

	s2 := NewSettingsService(repo.NewSettingsRepo(db), stubCounter{n: 3})
	v2 := s2.View()
	if v2.CategoryCount != 3 || v2.SetupComplete {
		t.Fatalf("%+v", v2)
	}
	s3 := NewSettingsService(repo.NewSettingsRepo(db), stubCounter{n: 4, err: errCount})
	if s3.View().CategoryCount != 0 {
		t.Fatal(s3.View())
	}
}

var errCount = errString("count failed")

type errString string

func (e errString) Error() string { return string(e) }

func TestFormatOSSErrorInvalidAccessKeyId(t *testing.T) {
	raw := errors.New(`oss: service returned error: StatusCode=403, ErrorCode=InvalidAccessKeyId, ErrorMessage="The OSS Access Key Id you provided does not exist in our records.", RequestId=ABC`)
	msg := FormatOSSError(raw)
	if !strings.Contains(msg, "AccessKey ID") || strings.Contains(msg, "RequestId=") {
		t.Fatalf("msg=%q", msg)
	}
	msg2 := FormatOSSError(errors.New(`oss: service returned error: StatusCode=404, ErrorCode=NoSuchBucket, ErrorMessage="x", RequestId=1`))
	if !strings.Contains(msg2, "Bucket") {
		t.Fatalf("msg2=%q", msg2)
	}
}

func TestPlaceholderOSSNotReady(t *testing.T) {
	msg := ossPlaceholderReason(repo.SettingsRow{
		Endpoint:        "oss-cn-beijing.aliyuncs.com",
		Bucket:          "your-bucket-name",
		AccessKeyID:     "LTAI5txxxxxxxx",
		AccessKeySecret: "xxxxxxxxxxxxxxxx",
		URLPrefix:       "https://your-bucket-name.oss-cn-beijing.aliyuncs.com/",
	})
	if msg == "" {
		t.Fatal("expected placeholder reason")
	}

	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := repo.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := NewSettingsService(repo.NewSettingsRepo(db), nil)
	st := s.SetupStatus()
	if st.OssReady || st.SetupComplete {
		t.Fatalf("empty settings must not be ready: %+v", st)
	}
	if st.OssError == "" || !strings.Contains(st.OssError, "尚未配置") {
		t.Fatalf("ossError=%q", st.OssError)
	}
}
