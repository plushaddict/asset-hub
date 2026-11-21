package service

import (
	"path/filepath"
	"testing"

	"assethub/internal/model"
	"assethub/internal/repo"
)

func TestCreateAndUpdateAssetTags(t *testing.T) {
	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = repo.Migrate(db); err != nil {
		t.Fatal(err)
	}

	catRepo := repo.NewCategoryRepo(db)
	tagRepo := repo.NewTagRepo(db)
	assetRepo := repo.NewAssetRepo(db)
	svc := NewAssetService(assetRepo, repo.NewAssetFileRepo(db), catRepo, tagRepo, NewSettingsService(nil, nil, nil))

	catID, err := catRepo.Insert(&model.Category{Name: "图标", OssPrefix: "icons/", Sort: 0})
	if err != nil {
		t.Fatal(err)
	}
	redID, err := tagRepo.Insert(&model.Tag{Name: "红"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = svc.CreateAsset("logo", 999, nil, nil, "", "admin"); err == nil || err.Error() != "分类不存在" {
		t.Fatalf("missing category: %v", err)
	}
	if _, err = svc.CreateAsset("logo", catID, []int64{42}, nil, "", "admin"); err == nil || err.Error() != "标签不存在" {
		t.Fatalf("missing tag: %v", err)
	}

	a, err := svc.CreateAsset("logo", catID, []int64{redID, redID}, []string{"  蓝  ", "红", "  "}, "note", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if a.CategoryName != "图标" || len(a.Tags) != 2 {
		t.Fatalf("created = name %q tags %+v", a.CategoryName, a.Tags)
	}
	names := map[string]bool{}
	for _, tag := range a.Tags {
		names[tag.Name] = true
	}
	if !names["红"] || !names["蓝"] {
		t.Fatalf("tags = %+v", a.Tags)
	}

	dict, err := tagRepo.List()
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.UpdateAsset(a.ID, "logo", catID, false, nil, []string{"不该出现"}, "note2", 3); err != nil {
		t.Fatal(err)
	}
	got, err := tagRepo.ListByAssetID(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("omit tags changed associations: %+v", got)
	}
	dictAfter, err := tagRepo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(dictAfter) != len(dict) {
		t.Fatalf("omit tags upserted names: before %d after %d", len(dict), len(dictAfter))
	}
	stored, err := assetRepo.GetByID(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Remark != "note2" || stored.Sort != 3 || len(stored.Tags) != 2 {
		t.Fatalf("updated asset = %+v", stored)
	}

	if err = svc.UpdateAsset(a.ID, "logo", catID, true, []int64{}, []string{}, "note2", 3); err != nil {
		t.Fatal(err)
	}
	cleared, err := tagRepo.ListByAssetID(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared) != 0 {
		t.Fatalf("clear tags = %+v", cleared)
	}
}
