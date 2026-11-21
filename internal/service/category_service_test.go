package service

import (
	"path/filepath"
	"testing"

	"assethub/internal/repo"
)

func TestNormalizePrefix(t *testing.T) {
	got, err := NormalizePrefix(" icons ")
	if err != nil || got != "icons/" {
		t.Fatalf("trim+slash: got %q err %v", got, err)
	}
	got, err = NormalizePrefix("icons/")
	if err != nil || got != "icons/" {
		t.Fatalf("keep slash: got %q err %v", got, err)
	}
	if _, err = NormalizePrefix("  "); err == nil || err.Error() != "ossPrefix 不能为空" {
		t.Fatalf("empty: %v", err)
	}
}

func TestCategoryUniqueDeleteAndTagUnique(t *testing.T) {
	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = repo.Migrate(db); err != nil {
		t.Fatal(err)
	}

	catRepo := repo.NewCategoryRepo(db)
	var counter CategoryCounter = catRepo
	cats := NewCategoryService(catRepo)
	created, err := cats.Create(" 图标 ", "icons", 1)
	if err != nil {
		t.Fatal(err)
	}
	if created.OssPrefix != "icons/" {
		t.Fatalf("prefix = %q", created.OssPrefix)
	}
	if _, err = cats.Create("图标", "covers", 2); err == nil || err.Error() != "分类名称已存在" {
		t.Fatalf("category unique: %v", err)
	}
	n, err := counter.Count()
	if err != nil || n != 1 {
		t.Fatalf("Count=%d err=%v", n, err)
	}

	if _, err = db.Exec(`INSERT INTO asset(asset_name, category_id) VALUES ('logo', ?)`, created.ID); err != nil {
		t.Fatal(err)
	}
	if err = cats.Delete(created.ID); err == nil || err.Error() != "该分类下仍有素材，无法删除" {
		t.Fatalf("delete guarded: %v", err)
	}

	tags := NewTagService(repo.NewTagRepo(db))
	if _, err = tags.Create("红"); err != nil {
		t.Fatal(err)
	}
	if _, err = tags.Create(" 红 "); err == nil || err.Error() != "标签名称已存在" {
		t.Fatalf("tag unique: %v", err)
	}
	if _, err = tags.Create("   "); err == nil || err.Error() != "标签名称不能为空" {
		t.Fatalf("tag empty: %v", err)
	}
}
