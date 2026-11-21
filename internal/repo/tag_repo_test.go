package repo

import (
	"path/filepath"
	"testing"

	"assethub/internal/model"
)

func TestReplaceAssetTags(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}

	catID, err := NewCategoryRepo(db).Insert(&model.Category{Name: "图标", OssPrefix: "icons/", Sort: 0})
	if err != nil {
		t.Fatal(err)
	}
	tags := NewTagRepo(db)
	tagA, err := tags.Insert(&model.Tag{Name: "红"})
	if err != nil {
		t.Fatal(err)
	}
	tagB, err := tags.Insert(&model.Tag{Name: "蓝"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO asset(asset_name, category_id) VALUES ('logo', ?)`, catID)
	if err != nil {
		t.Fatal(err)
	}
	assetID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO asset_tag(asset_id, tag_id) VALUES (?, ?), (?, ?)`,
		assetID, tagA, assetID, tagB); err != nil {
		t.Fatal(err)
	}

	if err = tags.ReplaceAssetTags(assetID, []int64{tagB}); err != nil {
		t.Fatal(err)
	}
	got, err := tags.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ListByAssetID len=%d, want 1", len(got))
	}
	if got[0].ID != tagB || got[0].Name != "蓝" {
		t.Fatalf("remaining tag = %+v, want id=%d name=蓝", got[0], tagB)
	}
}
