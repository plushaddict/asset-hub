package service

import (
	"path/filepath"
	"strings"
	"testing"

	"assethub/internal/model"
	"assethub/internal/repo"
)

func TestSlugFromDisplayName(t *testing.T) {
	got := SlugFromDisplayName("百度搜索")
	if !nameRe.MatchString(got) || got == "" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "百度") {
		t.Fatalf("must be latin slug: %q", got)
	}
	if !(strings.Contains(got, "bai") || strings.Contains(got, "du") || strings.Contains(got, "sou") || strings.Contains(got, "suo")) {
		t.Fatalf("expected pinyin fragments in %q", got)
	}
	if got := SlugFromDisplayName("！！！"); got != "asset" {
		t.Fatalf("symbol-only: %q", got)
	}
	if got := SlugFromDisplayName("！百度"); got[0] == '_' {
		t.Fatalf("leading underscore: %q", got)
	}
	long := strings.Repeat("测", 80)
	got = SlugFromDisplayName(long)
	if len(got) > 64 || !nameRe.MatchString(got) {
		t.Fatalf("truncate: %q len=%d", got, len(got))
	}
	if got := SlugFromDisplayName("logo"); got != "logo" {
		t.Fatalf("ascii: got %q want logo", got)
	}
	got = SlugFromDisplayName("百度OK")
	if !nameRe.MatchString(got) {
		t.Fatalf("mixed: invalid slug %q", got)
	}
	lower := strings.ToLower(got)
	if !(strings.Contains(lower, "bai") || strings.Contains(lower, "du")) || !strings.Contains(lower, "ok") {
		t.Fatalf("mixed missing pinyin+ascii: %q", got)
	}
	if got := SlugFromDisplayName("123"); got != "123" || !nameRe.MatchString(got) {
		t.Fatalf("digits: got %q", got)
	}
}

func TestAllocateUniqueAssetNameSuffixAndLength(t *testing.T) {
	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = repo.Migrate(db); err != nil {
		t.Fatal(err)
	}

	catRepo := repo.NewCategoryRepo(db)
	catID, err := catRepo.Insert(&model.Category{Name: "图标", OssPrefix: "icons/", Sort: 0})
	if err != nil {
		t.Fatal(err)
	}
	assetRepo := repo.NewAssetRepo(db)
	svc := NewAssetService(assetRepo, repo.NewAssetFileRepo(db), catRepo, repo.NewTagRepo(db), NewSettingsService(nil, nil))

	base := "my_slug"
	if _, err = assetRepo.Insert(&model.Asset{AssetName: base, CategoryID: catID, CreateBy: "t"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.allocateUniqueAssetName(base)
	if err != nil {
		t.Fatal(err)
	}
	if got != "my_slug_2" {
		t.Fatalf("suffix: got %q want my_slug_2", got)
	}

	longBase := strings.Repeat("a", 64)
	if _, err = assetRepo.Insert(&model.Asset{AssetName: longBase, CategoryID: catID, CreateBy: "t"}); err != nil {
		t.Fatal(err)
	}
	got, err = svc.allocateUniqueAssetName(longBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > 64 || !nameRe.MatchString(got) {
		t.Fatalf("long base: got %q len=%d", got, len(got))
	}
	if got == longBase {
		t.Fatalf("expected suffixed name, got %q", got)
	}
}