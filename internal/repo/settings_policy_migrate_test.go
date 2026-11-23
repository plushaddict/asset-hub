package repo

import (
	"path/filepath"
	"testing"
)

func TestMigrateSettingsPolicyColumnsFreshDB(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"own_oss_link_mode", "reupload_delete_source", "delete_remove_oss"} {
		var n int
		q := `SELECT COUNT(*) FROM pragma_table_info('system_settings') WHERE name=?`
		if err := db.QueryRow(q, col).Scan(&n); err != nil || n != 1 {
			t.Fatalf("column %s missing: err=%v n=%d", col, err, n)
		}
	}
	r := NewSettingsRepo(db)
	if err := r.Upsert(SettingsRow{
		Endpoint:        "ep",
		Bucket:          "b",
		AccessKeyID:     "id",
		AccessKeySecret: "sec",
		URLPrefix:       "https://x",
	}); err != nil {
		t.Fatal(err)
	}
	row, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row.OwnOssLinkMode != "register" || row.ReuploadDeleteSource || !row.DeleteRemoveOss {
		t.Fatalf("policy defaults: %+v", row)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacySettingsPolicyColumns(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE system_settings (
    id                INTEGER PRIMARY KEY,
    endpoint          TEXT NOT NULL,
    bucket            TEXT NOT NULL,
    access_key_id     TEXT NOT NULL,
    access_key_secret TEXT NOT NULL,
    private           INTEGER NOT NULL,
    signed_expire     INTEGER NOT NULL,
    url_prefix        TEXT NOT NULL,
    business_token    TEXT NOT NULL DEFAULT '',
    update_time       TEXT NOT NULL
);
INSERT INTO system_settings (
    id, endpoint, bucket, access_key_id, access_key_secret,
    private, signed_expire, url_prefix, business_token, update_time
) VALUES (1, 'ep', 'b', 'id', 'sec', 0, 3600, 'https://x', '', '2020-01-01 00:00:00');`)
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('system_settings') WHERE name='own_oss_link_mode'`).Scan(&before); err != nil || before != 0 {
		t.Fatalf("pre-migrate should lack own_oss_link_mode: %v n=%d", err, before)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"own_oss_link_mode", "reupload_delete_source", "delete_remove_oss"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('system_settings') WHERE name=?`, col).Scan(&n); err != nil || n != 1 {
			t.Fatalf("post-migrate column %s: err=%v n=%d", col, err, n)
		}
	}
	r := NewSettingsRepo(db)
	row, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row.OwnOssLinkMode != "register" || row.ReuploadDeleteSource || !row.DeleteRemoveOss {
		t.Fatalf("legacy row policy defaults: %+v", row)
	}
}

func TestSettingsPolicyRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := NewSettingsRepo(db)
	if err := r.Upsert(SettingsRow{
		Endpoint:               "ep",
		Bucket:                 "b",
		AccessKeyID:            "id",
		AccessKeySecret:        "sec",
		URLPrefix:              "https://x",
		OwnOssLinkMode:         "link",
		ReuploadDeleteSource:   true,
		DeleteRemoveOss:        false,
	}); err != nil {
		t.Fatal(err)
	}
	row, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if row.OwnOssLinkMode != "link" || !row.ReuploadDeleteSource || row.DeleteRemoveOss {
		t.Fatalf("got %+v", row)
	}
}