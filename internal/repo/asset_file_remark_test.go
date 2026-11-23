package repo

import (
	"path/filepath"
	"testing"

	"assethub/internal/model"
)

func TestMigrateLegacyDBAddsAssetFileRemark(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 模拟升级前库：asset_file 无 remark
	_, err = db.Exec(`
CREATE TABLE asset (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_name TEXT NOT NULL,
    category_id INTEGER NOT NULL DEFAULT 1,
    remark TEXT NOT NULL DEFAULT '',
    sort INTEGER NOT NULL DEFAULT 0,
    create_by TEXT NOT NULL DEFAULT '',
    create_time TEXT NOT NULL DEFAULT '',
    update_time TEXT NOT NULL DEFAULT '',
    del_flag INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE asset_file (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    suffix TEXT NOT NULL,
    file_size INTEGER NOT NULL DEFAULT 0,
    width INTEGER,
    height INTEGER,
    oss_object_key TEXT NOT NULL,
    is_primary INTEGER NOT NULL DEFAULT 0,
    create_time TEXT NOT NULL DEFAULT '',
    del_flag INTEGER NOT NULL DEFAULT 0
);`)
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('asset_file') WHERE name='remark'`).Scan(&before); err != nil || before != 0 {
		t.Fatalf("pre-migrate should lack remark: %v n=%d", err, before)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var after int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('asset_file') WHERE name='remark'`).Scan(&after); err != nil || after != 1 {
		t.Fatalf("post-migrate remark missing: %v n=%d", err, after)
	}
	_, err = db.Exec(`INSERT INTO asset(asset_name, category_id, remark, sort, create_by, create_time, update_time)
		VALUES ('legacy',1,'',0,'a','t','t')`)
	if err != nil {
		t.Fatal(err)
	}
	fr := NewAssetFileRepo(db)
	id, err := fr.Insert(&model.AssetFile{
		AssetID: 1, FileName: "old.png", Suffix: "png", FileSize: 2,
		ObjectKey: "k/old.png", IsPrimary: 1, Remark: "legacy-note",
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := fr.ListByAssetID(1)
	if err != nil || len(list) != 1 || list[0].Remark != "legacy-note" {
		t.Fatalf("ListByAssetID %+v err %v", list, err)
	}
	pf, err := fr.PrimaryFile(1)
	if err != nil || pf.ID != id || pf.Remark != "legacy-note" {
		t.Fatalf("PrimaryFile %+v err %v", pf, err)
	}
}

func TestMigrateAddsAssetFileRemark(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('asset_file') WHERE name='remark'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("remark column missing: %v n=%d", err, n)
	}
	// 二次 Migrate 幂等
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
}

func TestAssetFileRemarkRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO category(name, oss_prefix, sort, create_time, update_time) VALUES ('c','icon/',0,'t','t')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO asset(asset_name, category_id, remark, sort, create_by, create_time, update_time)
		VALUES ('logo',1,'',0,'a','t','t')`)
	if err != nil {
		t.Fatal(err)
	}
	fr := NewAssetFileRepo(db)
	id, err := fr.Insert(&model.AssetFile{
		AssetID: 1, FileName: "logo.png", Suffix: "png", FileSize: 1,
		ObjectKey: "icon/logo.png", IsPrimary: 1, Remark: "彩色",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := fr.GetByID(id)
	if err != nil || got.Remark != "彩色" {
		t.Fatalf("got %+v err %v", got, err)
	}
	if err := fr.UpdateRemark(id, "白底"); err != nil {
		t.Fatal(err)
	}
	got, err = fr.GetByID(id)
	if err != nil || got.Remark != "白底" {
		t.Fatalf("after update %+v err %v", got, err)
	}
}