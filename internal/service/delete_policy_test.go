package service

import (
	"path/filepath"
	"testing"

	"assethub/internal/model"
	"assethub/internal/repo"
)

func newDeletePolicyAssetSvc(t *testing.T, deleteRemoveOss bool, store objectStore) (*AssetService, *repo.AssetRepo, *repo.AssetFileRepo, int64) {
	t.Helper()
	db, err := repo.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = repo.Migrate(db); err != nil {
		t.Fatal(err)
	}
	catRepo := repo.NewCategoryRepo(db)
	fileRepo := repo.NewAssetFileRepo(db)
	assetRepo := repo.NewAssetRepo(db)
	catID, err := catRepo.Insert(&model.Category{Name: "图标", OssPrefix: "icons/", Sort: 0})
	if err != nil {
		t.Fatal(err)
	}
	settings := &SettingsService{
		hasRow: true,
		row: repo.SettingsRow{
			OwnOssLinkMode:  "register",
			DeleteRemoveOss: deleteRemoveOss,
		},
	}
	svc := NewAssetService(assetRepo, fileRepo, catRepo, repo.NewTagRepo(db), settings)
	svc.store = store
	a, err := svc.CreateAsset("logo", "logo", catID, nil, nil, "", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return svc, assetRepo, fileRepo, a.ID
}

func insertTestAssetFile(t *testing.T, fileRepo *repo.AssetFileRepo, assetID int64, suffix, key string, primary int) int64 {
	t.Helper()
	id, err := fileRepo.Insert(&model.AssetFile{
		AssetID:   assetID,
		FileName:  "logo." + suffix,
		Suffix:    suffix,
		ObjectKey: key,
		IsPrimary: primary,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDeleteAsset_SkipOSSWhenPolicyFalse(t *testing.T) {
	svc, assetRepo, fileRepo, assetID := newDeletePolicyAssetSvc(t, false, nil)
	insertTestAssetFile(t, fileRepo, assetID, "png", "icons/logo.png", 1)

	if err := svc.DeleteAsset(assetID); err != nil {
		t.Fatalf("DeleteAsset: %v", err)
	}
	if _, err := assetRepo.GetByID(assetID); err == nil {
		t.Fatal("asset still present")
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("files still present: %+v", list)
	}
}

func TestDeleteFile_SkipOSSWhenPolicyFalse_PromotesPrimary(t *testing.T) {
	oss := &fakeOSS{ready: true}
	svc, _, fileRepo, assetID := newDeletePolicyAssetSvc(t, false, oss)
	primaryID := insertTestAssetFile(t, fileRepo, assetID, "png", "icons/logo.png", 1)
	nextID := insertTestAssetFile(t, fileRepo, assetID, "jpg", "icons/logo.jpg", 0)

	if err := svc.DeleteFile(assetID, primaryID); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if len(oss.deletes) != 0 {
		t.Fatalf("OSS deletes = %v", oss.deletes)
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != nextID || list[0].IsPrimary != 1 {
		t.Fatalf("after promote: %+v", list)
	}
}

func TestDeleteAsset_EmptyKeyDoesNotRequireOSSWhenPolicyTrue(t *testing.T) {
	svc, assetRepo, fileRepo, assetID := newDeletePolicyAssetSvc(t, true, nil)
	insertTestAssetFile(t, fileRepo, assetID, "png", "", 1)

	if err := svc.DeleteAsset(assetID); err != nil {
		t.Fatalf("DeleteAsset: %v", err)
	}
	if _, err := assetRepo.GetByID(assetID); err == nil {
		t.Fatal("asset still present")
	}
}

func TestDeleteFile_EmptyKeyDoesNotRequireOSSWhenPolicyTrue(t *testing.T) {
	svc, _, fileRepo, assetID := newDeletePolicyAssetSvc(t, true, nil)
	fileID := insertTestAssetFile(t, fileRepo, assetID, "png", "  ", 1)

	if err := svc.DeleteFile(assetID, fileID); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("file still present: %+v", list)
	}
}

func TestDeleteAsset_WithKeyRequiresOSSWhenPolicyTrue(t *testing.T) {
	svc, assetRepo, fileRepo, assetID := newDeletePolicyAssetSvc(t, true, nil)
	insertTestAssetFile(t, fileRepo, assetID, "png", "icons/logo.png", 1)

	err := svc.DeleteAsset(assetID)
	if err == nil || err.Error() != ossNotReadyMsg {
		t.Fatalf("err=%v", err)
	}
	if _, err = assetRepo.GetByID(assetID); err != nil {
		t.Fatalf("asset should remain: %v", err)
	}
}

func TestDeleteFile_WithKeyRequiresOSSWhenPolicyTrue(t *testing.T) {
	svc, _, fileRepo, assetID := newDeletePolicyAssetSvc(t, true, nil)
	fileID := insertTestAssetFile(t, fileRepo, assetID, "png", "icons/logo.png", 1)

	err := svc.DeleteFile(assetID, fileID)
	if err == nil || err.Error() != ossNotReadyMsg {
		t.Fatalf("err=%v", err)
	}
	got, err := fileRepo.GetByID(fileID)
	if err != nil || got.ID != fileID {
		t.Fatalf("file should remain: %+v %v", got, err)
	}
}

func TestDeleteAsset_RemovesOSSWhenPolicyTrue(t *testing.T) {
	oss := &fakeOSS{ready: true}
	svc, assetRepo, fileRepo, assetID := newDeletePolicyAssetSvc(t, true, oss)
	insertTestAssetFile(t, fileRepo, assetID, "png", "icons/logo.png", 1)
	insertTestAssetFile(t, fileRepo, assetID, "jpg", "", 0)

	if err := svc.DeleteAsset(assetID); err != nil {
		t.Fatalf("DeleteAsset: %v", err)
	}
	if len(oss.deletes) != 1 || oss.deletes[0] != "icons/logo.png" {
		t.Fatalf("OSS deletes = %v", oss.deletes)
	}
	if _, err := assetRepo.GetByID(assetID); err == nil {
		t.Fatal("asset still present")
	}
}

func TestDeleteFile_RemovesOSSWhenPolicyTrue(t *testing.T) {
	oss := &fakeOSS{ready: true}
	svc, _, fileRepo, assetID := newDeletePolicyAssetSvc(t, true, oss)
	fileID := insertTestAssetFile(t, fileRepo, assetID, "png", "icons/logo.png", 1)

	if err := svc.DeleteFile(assetID, fileID); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if len(oss.deletes) != 1 || oss.deletes[0] != "icons/logo.png" {
		t.Fatalf("OSS deletes = %v", oss.deletes)
	}
}
