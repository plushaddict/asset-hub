package repo

import (
	"path/filepath"
	"testing"

	"assethub/internal/model"
)

func TestMigrateLegacyDBAddsDisplayNameAndBackfill(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE category (
  id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL,
  oss_prefix TEXT NOT NULL, sort INTEGER NOT NULL DEFAULT 0,
  create_time TEXT NOT NULL DEFAULT '', update_time TEXT NOT NULL DEFAULT ''
);
CREATE TABLE asset (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset_name TEXT NOT NULL, category_id INTEGER NOT NULL,
  remark TEXT NOT NULL DEFAULT '', sort INTEGER NOT NULL DEFAULT 0,
  create_by TEXT NOT NULL DEFAULT '', create_time TEXT NOT NULL DEFAULT '',
  update_time TEXT NOT NULL DEFAULT '', del_flag INTEGER NOT NULL DEFAULT 0
);
INSERT INTO category(name, oss_prefix, sort) VALUES ('c','icon/',0);
INSERT INTO asset(asset_name, category_id) VALUES ('logo_book', 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('asset') WHERE name='display_name'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("display_name missing: %v n=%d", err, n)
	}
	var dn string
	if err = db.QueryRow(`SELECT display_name FROM asset WHERE asset_name='logo_book'`).Scan(&dn); err != nil || dn != "logo_book" {
		t.Fatalf("backfill got %q err=%v", dn, err)
	}
}

func TestPageListKeywordMatchesDisplayNameAndAssetName(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "kw.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
	cats := NewCategoryRepo(db)
	catID, err := cats.Insert(&model.Category{Name: "icons", OssPrefix: "icon/"})
	if err != nil {
		t.Fatal(err)
	}
	assets := NewAssetRepo(db)
	if _, err = assets.Insert(&model.Asset{
		AssetName:   "baidu_sousuo",
		DisplayName: "百度搜索",
		CategoryID:  catID,
	}); err != nil {
		t.Fatal(err)
	}
	_, total, err := assets.PageList("百度", 0, nil, 1, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if total < 1 {
		t.Fatalf("expected total>=1 for display name keyword, got %d", total)
	}
	_, total, err = assets.PageList("baidu_sousuo", 0, nil, 1, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if total < 1 {
		t.Fatalf("expected total>=1 for asset name keyword, got %d", total)
	}
}