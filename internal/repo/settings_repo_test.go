package repo

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"assethub/internal/config"
)

func TestEnsureSeededDoesNotOverwrite(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := NewSettingsRepo(db)
	if _, err := r.Get(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("empty get: %v", err)
	}
	cfg := &config.Config{}
	cfg.OSS.Endpoint = "oss-cn-beijing.aliyuncs.com"
	cfg.OSS.Bucket = "bkt"
	cfg.OSS.AccessKeyID = "LTAI12345678"
	cfg.OSS.AccessKeySecret = "secret"
	cfg.OSS.Private = true
	cfg.OSS.SignedExpire = 0
	cfg.OSS.URLPrefix = "https://bkt.example.com"
	cfg.Server.BusinessToken = "tok"
	if err := r.EnsureSeeded(cfg); err != nil {
		t.Fatal(err)
	}
	row, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row.Bucket != "bkt" || row.BusinessToken != "tok" || !row.Private || row.SignedExpire != 3600 || row.UpdateTime == "" {
		t.Fatalf("%+v", row)
	}
	cfg.OSS.Bucket = "other"
	cfg.Server.BusinessToken = "new"
	if err := r.EnsureSeeded(cfg); err != nil {
		t.Fatal(err)
	}
	row2, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row2.Bucket != "bkt" || row2.BusinessToken != "tok" {
		t.Fatalf("seed overwrote: %+v", row2)
	}
	row2.Bucket = "changed"
	row2.BusinessToken = ""
	if err := r.Upsert(*row2); err != nil {
		t.Fatal(err)
	}
	row3, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row3.Bucket != "changed" || row3.BusinessToken != "" || row3.AccessKeySecret != "secret" {
		t.Fatalf("%+v", row3)
	}
}
