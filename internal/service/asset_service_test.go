package service

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"assethub/internal/model"
	"assethub/internal/repo"
)

func TestNormalizeFileRemark(t *testing.T) {
	s, err := normalizeFileRemark("  彩色  ")
	if err != nil || s != "彩色" {
		t.Fatalf("%q %v", s, err)
	}
	long := strings.Repeat("字", 201)
	if _, err := normalizeFileRemark(long); err == nil {
		t.Fatal("expected length error")
	}
}

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
	svc := NewAssetService(assetRepo, repo.NewAssetFileRepo(db), catRepo, tagRepo, NewSettingsService(nil, nil))

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

func TestUploadFileFromURL_BadURLBeforeOSS(t *testing.T) {
	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = repo.Migrate(db)
	catRepo := repo.NewCategoryRepo(db)
	catID, _ := catRepo.Insert(&model.Category{Name: "图标", OssPrefix: "icons/", Sort: 0})
	svc := NewAssetService(repo.NewAssetRepo(db), repo.NewAssetFileRepo(db), catRepo, repo.NewTagRepo(db), NewSettingsService(nil, nil))
	a, err := svc.CreateAsset("logo", catID, nil, nil, "", "admin")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.UploadFileFromURL(a.ID, "ftp://x/a.ico", "")
	if err == nil || !strings.Contains(err.Error(), "http") {
		t.Fatalf("err=%v", err)
	}
}

func TestUploadFileFromURL_HTTP500BeforeOSS(t *testing.T) {
	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = repo.Migrate(db); err != nil {
		t.Fatal(err)
	}
	catRepo := repo.NewCategoryRepo(db)
	fileRepo := repo.NewAssetFileRepo(db)
	catID, err := catRepo.Insert(&model.Category{Name: "图标", OssPrefix: "icons/", Sort: 0})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewAssetService(repo.NewAssetRepo(db), fileRepo, catRepo, repo.NewTagRepo(db), NewSettingsService(nil, nil))
	a, err := svc.CreateAsset("logo", catID, nil, nil, "", "admin")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err = svc.UploadFileFromURL(a.ID, srv.URL+"/a.png", "")
	if err == nil || !strings.Contains(err.Error(), "远程文件下载失败") || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err=%v", err)
	}
	list, err := fileRepo.ListByAssetID(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("file rows = %d, want 0", len(list))
	}
}
