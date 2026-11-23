package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"assethub/internal/model"
	"assethub/internal/repo"
)

type fakeOSS struct {
	ready     bool
	heads     map[string]int64
	headErr   error
	puts      []string
	putErr    error
	deletes   []string
	deleteErr error
	urlPrefix string
}

func (f *fakeOSS) Ready() bool { return f != nil && f.ready }

func (f *fakeOSS) HeadObjectMeta(key string) (int64, error) {
	if f.headErr != nil {
		return 0, f.headErr
	}
	if f.heads != nil {
		if sz, ok := f.heads[key]; ok {
			return sz, nil
		}
	}
	return 0, fmt.Errorf("%w: %s", ErrOSSObjectNotFound, key)
}

func (f *fakeOSS) PutObject(key string, r io.ReadSeeker, size int64, contentType string) error {
	f.puts = append(f.puts, key)
	return f.putErr
}

func (f *fakeOSS) DeleteObject(key string) error {
	if key == "" {
		return nil
	}
	f.deletes = append(f.deletes, key)
	return f.deleteErr
}

func (f *fakeOSS) DeleteObjects(keys []string) error {
	var first error
	for _, k := range keys {
		if err := f.DeleteObject(k); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (f *fakeOSS) GenObjectKey(prefix, assetName, suffix string) string {
	return (&OSSService{}).GenObjectKey(prefix, assetName, suffix)
}

func (f *fakeOSS) URL(key string) (string, error) {
	prefix := f.urlPrefix
	if prefix == "" {
		prefix = "https://example.test"
	}
	return strings.TrimRight(prefix, "/") + "/" + key, nil
}

func newPolicyAssetSvc(t *testing.T, mode string, reuploadDel bool, endpoint, bucket string, store objectStore) (*AssetService, *repo.AssetFileRepo, int64) {
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
	catID, err := catRepo.Insert(&model.Category{Name: "图标", OssPrefix: "icons/", Sort: 0})
	if err != nil {
		t.Fatal(err)
	}
	settings := &SettingsService{
		hasRow: true,
		row: repo.SettingsRow{
			Endpoint:             endpoint,
			Bucket:               bucket,
			OwnOssLinkMode:       mode,
			ReuploadDeleteSource: reuploadDel,
			DeleteRemoveOss:      true,
		},
	}
	svc := NewAssetService(repo.NewAssetRepo(db), fileRepo, catRepo, repo.NewTagRepo(db), settings)
	svc.store = store
	a, err := svc.CreateAsset("logo", "logo", catID, nil, nil, "", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return svc, fileRepo, a.ID
}

func TestShouldDeleteReuploadSource(t *testing.T) {
	cases := []struct {
		del  bool
		old  string
		new  string
		want bool
	}{
		{true, "a.png", "b.png", true},
		{true, "same.png", "same.png", false},
		{false, "a.png", "b.png", false},
		{true, "", "b.png", false},
		{true, "a.png", "", false},
	}
	for _, c := range cases {
		if got := shouldDeleteReuploadSource(c.del, c.old, c.new); got != c.want {
			t.Fatalf("del=%v old=%q new=%q got=%v want=%v", c.del, c.old, c.new, got, c.want)
		}
	}
}

func TestUploadFileFromURL_RegisterUsesOriginalKeyNoPut(t *testing.T) {
	oss := &fakeOSS{
		ready: true,
		heads: map[string]int64{"icons/old-logo.png": 2048},
	}
	svc, fileRepo, assetID := newPolicyAssetSvc(t, "register", false, "oss-cn-beijing.aliyuncs.com", "my-bucket", oss)
	src := "https://my-bucket.oss-cn-beijing.aliyuncs.com/icons/old-logo.png?Expires=1&Signature=x"

	f, err := svc.UploadFileFromURL(assetID, src, "  直登  ")
	if err != nil {
		t.Fatal(err)
	}
	if f.ObjectKey != "icons/old-logo.png" {
		t.Fatalf("objectKey=%q", f.ObjectKey)
	}
	if f.FileName != "old-logo.png" || f.Suffix != "png" || f.FileSize != 2048 {
		t.Fatalf("file=%+v", f)
	}
	if f.Width != nil || f.Height != nil {
		t.Fatalf("width/height should be nil: %+v", f)
	}
	if f.IsPrimary != 1 || f.Remark != "直登" {
		t.Fatalf("primary/remark: %+v", f)
	}
	if len(oss.puts) != 0 {
		t.Fatalf("puts=%v, want none", oss.puts)
	}
	if len(oss.deletes) != 0 {
		t.Fatalf("deletes=%v, want none", oss.deletes)
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ObjectKey != "icons/old-logo.png" {
		t.Fatalf("rows=%+v", list)
	}
}

func TestUploadFileFromURL_RegisterHeadNotFoundNoInsert(t *testing.T) {
	oss := &fakeOSS{
		ready:   true,
		headErr: fmt.Errorf("%w: gone", ErrOSSObjectNotFound),
	}
	svc, fileRepo, assetID := newPolicyAssetSvc(t, "register", false, "oss-cn-beijing.aliyuncs.com", "my-bucket", oss)
	src := "https://my-bucket.oss-cn-beijing.aliyuncs.com/icons/missing.png"

	_, err := svc.UploadFileFromURL(assetID, src, "")
	if err == nil || err.Error() != "OSS 对象不存在，无法直接登记" {
		t.Fatalf("err=%v", err)
	}
	if len(oss.puts) != 0 {
		t.Fatalf("puts=%v", oss.puts)
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("inserted %d rows", len(list))
	}
}

func TestUploadFileFromURL_OwnHostBadKey(t *testing.T) {
	oss := &fakeOSS{ready: true}
	svc, fileRepo, assetID := newPolicyAssetSvc(t, "register", false, "oss-cn-beijing.aliyuncs.com", "my-bucket", oss)
	src := "https://my-bucket.oss-cn-beijing.aliyuncs.com/"

	_, err := svc.UploadFileFromURL(assetID, src, "")
	if err == nil || !strings.Contains(err.Error(), "无法识别") {
		t.Fatalf("err=%v", err)
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("inserted %d rows", len(list))
	}
}

func TestUploadFileFromURL_ReuploadDeletesSourceWhenKeysDiffer(t *testing.T) {
	oss := &fakeOSS{ready: true, urlPrefix: "https://example.test"}
	body := []byte("png-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	svc, fileRepo, assetID := newPolicyAssetSvc(t, "reupload", true, u.Hostname(), "my-bucket", oss)
	src := srv.URL + "/my-bucket/old/path.png"

	f, err := svc.UploadFileFromURL(assetID, src, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.ObjectKey != "icons/logo.png" {
		t.Fatalf("new key=%q", f.ObjectKey)
	}
	if len(oss.puts) != 1 || oss.puts[0] != "icons/logo.png" {
		t.Fatalf("puts=%v", oss.puts)
	}
	if len(oss.deletes) != 1 || oss.deletes[0] != "old/path.png" {
		t.Fatalf("deletes=%v", oss.deletes)
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ObjectKey != "icons/logo.png" {
		t.Fatalf("rows=%+v", list)
	}
}

func TestUploadFileFromURL_ReuploadSkipsDeleteWhenKeysEqual(t *testing.T) {
	oss := &fakeOSS{ready: true}
	body := []byte("png-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	svc, _, assetID := newPolicyAssetSvc(t, "reupload", true, u.Hostname(), "my-bucket", oss)
	src := srv.URL + "/my-bucket/icons/logo.png"

	f, err := svc.UploadFileFromURL(assetID, src, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.ObjectKey != "icons/logo.png" {
		t.Fatalf("key=%q", f.ObjectKey)
	}
	if len(oss.deletes) != 0 {
		t.Fatalf("deletes=%v, want none when keys equal", oss.deletes)
	}
}

func TestUploadFileFromURL_ReuploadDeleteSourceFailKeepsInsert(t *testing.T) {
	oss := &fakeOSS{ready: true, deleteErr: errors.New("boom")}
	body := []byte("png-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	svc, fileRepo, assetID := newPolicyAssetSvc(t, "reupload", true, u.Hostname(), "my-bucket", oss)
	src := srv.URL + "/my-bucket/old/path.png"

	f, err := svc.UploadFileFromURL(assetID, src, "")
	if err == nil || !strings.Contains(err.Error(), "重传成功但删除原对象失败") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%v", err)
	}
	if f == nil || f.ObjectKey != "icons/logo.png" {
		t.Fatalf("file=%+v", f)
	}
	list, err := fileRepo.ListByAssetID(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want inserted row, got %d", len(list))
	}
}

func TestUploadFileFromURL_ExternalNeverDeletesSource(t *testing.T) {
	oss := &fakeOSS{ready: true}
	body := []byte("png-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	svc, _, assetID := newPolicyAssetSvc(t, "reupload", true, "oss-cn-beijing.aliyuncs.com", "my-bucket", oss)
	f, err := svc.UploadFileFromURL(assetID, srv.URL+"/a.png", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.ObjectKey != "icons/logo.png" {
		t.Fatalf("key=%q", f.ObjectKey)
	}
	if len(oss.deletes) != 0 {
		t.Fatalf("external must not delete source: %v", oss.deletes)
	}
}
