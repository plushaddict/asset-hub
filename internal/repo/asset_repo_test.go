package repo

import (
	"path/filepath"
	"testing"

	"assethub/internal/model"
)

func TestPageListTagAND(t *testing.T) {
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
	tag1, err := tags.Insert(&model.Tag{Name: "红"})
	if err != nil {
		t.Fatal(err)
	}
	tag2, err := tags.Insert(&model.Tag{Name: "蓝"})
	if err != nil {
		t.Fatal(err)
	}
	if tag1 != 1 || tag2 != 2 {
		t.Fatalf("tag ids = %d,%d, want 1,2", tag1, tag2)
	}

	assets := NewAssetRepo(db)
	assetA, err := assets.Insert(&model.Asset{AssetName: "logoA", CategoryID: catID, Remark: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	assetB, err := assets.Insert(&model.Asset{AssetName: "logoB", CategoryID: catID, Remark: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if err = tags.ReplaceAssetTags(assetA, []int64{tag1, tag2}); err != nil {
		t.Fatal(err)
	}
	if err = tags.ReplaceAssetTags(assetB, []int64{tag1}); err != nil {
		t.Fatal(err)
	}

	list, total, err := assets.PageList("", 0, []int64{1, 2}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("AND total=%d len=%d, want 1", total, len(list))
	}
	if list[0].ID != assetA || list[0].AssetName != "logoA" {
		t.Fatalf("AND hit %+v, want asset A", list[0])
	}
	if list[0].CategoryName != "图标" || list[0].CategoryID != catID {
		t.Fatalf("category = %d %q", list[0].CategoryID, list[0].CategoryName)
	}
	if len(list[0].Tags) != 2 || list[0].Tags[0].ID != tag1 || list[0].Tags[1].ID != tag2 {
		t.Fatalf("tags = %+v, want tag1+tag2", list[0].Tags)
	}

	both, bothTotal, err := assets.PageList("", 0, []int64{tag1}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if bothTotal != 2 || len(both) != 2 {
		t.Fatalf("single tag total=%d len=%d, want 2", bothTotal, len(both))
	}

	byName, nameTotal, err := assets.PageList("红", 0, nil, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if nameTotal != 0 || len(byName) != 0 {
		t.Fatalf("keyword must not match tag name, total=%d len=%d", nameTotal, len(byName))
	}
	byRemark, remarkTotal, err := assets.PageList("alpha", catID, nil, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if remarkTotal != 1 || len(byRemark) != 1 || byRemark[0].ID != assetA {
		t.Fatalf("remark filter = total %d list %v", remarkTotal, byRemark)
	}
}
