package repo

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestSettingsUpsertAndGet(t *testing.T) {
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
	if err := r.Upsert(SettingsRow{
		Endpoint:        "oss-cn-beijing.aliyuncs.com",
		Bucket:          "bkt",
		AccessKeyID:     "LTAI12345678",
		AccessKeySecret: "secret",
		Private:         true,
		SignedExpire:    3600,
		URLPrefix:       "https://bkt.example.com",
		BusinessToken:   "tok",
	}); err != nil {
		t.Fatal(err)
	}
	row, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row.Bucket != "bkt" || row.BusinessToken != "tok" || !row.Private || row.SignedExpire != 3600 || row.UpdateTime == "" {
		t.Fatalf("%+v", row)
	}
	row.Bucket = "changed"
	row.BusinessToken = ""
	if err := r.Upsert(*row); err != nil {
		t.Fatal(err)
	}
	row2, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row2.Bucket != "changed" || row2.BusinessToken != "" || row2.AccessKeySecret != "secret" {
		t.Fatalf("%+v", row2)
	}
}
