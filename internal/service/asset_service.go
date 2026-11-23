package service

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	_ "golang.org/x/image/webp"

	"assethub/internal/model"
	"assethub/internal/repo"
)

const (
	MaxUploadSize       = 20 << 20 // 20MB
	MaxFileRemarkRunes  = 200
	MaxDisplayNameRunes = 64
)

func normalizeDisplayName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("请填写显示名")
	}
	if utf8.RuneCountInString(s) > MaxDisplayNameRunes {
		return "", errors.New("显示名长度 1–64")
	}
	return s, nil
}

func normalizeFileRemark(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MaxFileRemarkRunes {
		return "", errors.New("说明不能超过 200 字")
	}
	return s, nil
}

var (
	allowedSuffix = map[string]bool{
		"png": true, "jpg": true, "jpeg": true,
		"svg": true, "webp": true, "ico": true, "gif": true,
	}
	suffixMime = map[string]string{
		"png":  "image/png",
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"svg":  "image/svg+xml",
		"webp": "image/webp",
		"ico":  "image/x-icon",
		"gif":  "image/gif",
	}
	nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

// objectStore 是素材服务用到的对象存储操作；测试可注入假实现。
type objectStore interface {
	Ready() bool
	HeadObjectMeta(key string) (size int64, err error)
	PutObject(key string, r io.ReadSeeker, size int64, contentType string) error
	DeleteObject(key string) error
	DeleteObjects(keys []string) error
	GenObjectKey(prefix, assetName, suffix string) string
	URL(key string) (string, error)
}

type AssetService struct {
	assetRepo    *repo.AssetRepo
	fileRepo     *repo.AssetFileRepo
	categoryRepo *repo.CategoryRepo
	tagRepo      *repo.TagRepo
	settings     *SettingsService
	store        objectStore
}

func NewAssetService(ar *repo.AssetRepo, fr *repo.AssetFileRepo, catRepo *repo.CategoryRepo, tagRepo *repo.TagRepo, settings *SettingsService) *AssetService {
	return &AssetService{
		assetRepo:    ar,
		fileRepo:     fr,
		categoryRepo: catRepo,
		tagRepo:      tagRepo,
		settings:     settings,
	}
}

func (s *AssetService) oss() objectStore {
	if s != nil && s.store != nil {
		return s.store
	}
	if s == nil || s.settings == nil {
		return nil
	}
	o := s.settings.OSS()
	if o == nil {
		return nil
	}
	return o
}

func (s *AssetService) requireOSS() (objectStore, error) {
	o := s.oss()
	if o == nil || !o.Ready() {
		return nil, errors.New(ossNotReadyMsg)
	}
	return o, nil
}

// ---------- 素材 ----------

func (s *AssetService) CreateAsset(displayName, assetName string, categoryID int64, tagIDs []int64, tagNames []string, remark, createBy string) (*model.Asset, error) {
	dn, err := normalizeDisplayName(displayName)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(assetName)
	if base == "" {
		base = SlugFromDisplayName(dn)
	} else if !nameRe.MatchString(base) {
		return nil, errors.New("素材名称只能包含字母、数字、下划线、短横线，长度 1-64")
	}
	finalName, err := s.allocateUniqueAssetName(base)
	if err != nil {
		return nil, err
	}
	cat, err := s.categoryRepo.Get(categoryID)
	if err != nil {
		return nil, mapNotFound(err, "分类不存在")
	}
	ids, err := s.resolveTagIDs(tagIDs, tagNames)
	if err != nil {
		return nil, err
	}
	a := &model.Asset{
		AssetName:    finalName,
		DisplayName:  dn,
		CategoryID:   cat.ID,
		CategoryName: cat.Name,
		Remark:       remark,
		CreateBy:     createBy,
	}
	id, err := s.assetRepo.Insert(a)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errors.New("素材名称已存在")
		}
		return nil, err
	}
	a.ID = id
	if err = s.tagRepo.ReplaceAssetTags(id, ids); err != nil {
		return nil, err
	}
	a.Tags, err = s.tagRepo.ListByAssetID(id)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *AssetService) GetAsset(id int64) (*model.Asset, error) {
	return s.assetRepo.GetByID(id)
}

func (s *AssetService) GetAssetByName(name string) (*model.Asset, error) {
	return s.assetRepo.GetByName(name)
}

// UpdateAsset 更新素材。tagsProvided 为 false 时不改标签关联；为 true 时用 tagIDs ∪ upsert(tagNames) 全量替换，空集合即清空。
func (s *AssetService) UpdateAsset(id int64, displayName, assetName string, categoryID int64, tagsProvided bool, tagIDs []int64, tagNames []string, remark string, sort int) error {
	a, err := s.assetRepo.GetByID(id)
	if err != nil {
		return mapNotFound(err, "素材不存在")
	}
	dn, err := normalizeDisplayName(displayName)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(assetName)
	if err = s.ensureName(name, id); err != nil {
		return err
	}
	if _, err = s.categoryRepo.Get(categoryID); err != nil {
		return mapNotFound(err, "分类不存在")
	}
	var ids []int64
	if tagsProvided {
		ids, err = s.resolveTagIDs(tagIDs, tagNames)
		if err != nil {
			return err
		}
	}
	a.DisplayName = dn
	a.AssetName = name
	a.CategoryID = categoryID
	a.Remark = remark
	a.Sort = sort
	if err = s.assetRepo.Update(a); err != nil {
		if isUniqueViolation(err) {
			return errors.New("素材名称已存在")
		}
		return err
	}
	if tagsProvided {
		return s.tagRepo.ReplaceAssetTags(id, ids)
	}
	return nil
}

func (s *AssetService) ensureName(name string, selfID int64) error {
	if !nameRe.MatchString(name) {
		return errors.New("素材名称只能包含字母、数字、下划线、短横线，长度 1-64")
	}
	old, err := s.assetRepo.GetByName(name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if old.ID != selfID {
		return errors.New("素材名称已存在")
	}
	return nil
}

// resolveTagIDs 合并已有标签 id 与按名称 upsert 的标签。名称先 TrimSpace，空白跳过。重复 id 只保留一次。
func (s *AssetService) resolveTagIDs(tagIDs []int64, tagNames []string) ([]int64, error) {
	seen := make(map[int64]struct{}, len(tagIDs)+len(tagNames))
	out := make([]int64, 0, len(tagIDs)+len(tagNames))
	for _, id := range tagIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		if _, err := s.tagRepo.Get(id); err != nil {
			return nil, mapNotFound(err, "标签不存在")
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, err := s.tagRepo.UpsertByName(name)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func (s *AssetService) PageList(keyword string, categoryID int64, tagIDs []int64, page, size int, sortBy, sortOrder string) ([]*model.Asset, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 12
	}
	list, total, err := s.assetRepo.PageList(keyword, categoryID, tagIDs, page, size, sortBy, sortOrder)
	if err != nil {
		return nil, 0, err
	}
	o := s.oss()
	if o == nil || !o.Ready() {
		return list, total, nil
	}
	for _, a := range list {
		f, err := s.fileRepo.PrimaryFile(a.ID)
		if err != nil {
			continue
		}
		if u, err := o.URL(f.ObjectKey); err == nil {
			a.PrimaryURL = u
		}
	}
	return list, total, nil
}

func (s *AssetService) DeleteAsset(id int64) error {
	if _, err := s.assetRepo.GetByID(id); err != nil {
		return mapNotFound(err, "素材不存在")
	}
	if s.deleteRemoveOssEnabled() {
		keys, err := s.fileRepo.ObjectKeys(id)
		if err != nil {
			return err
		}
		keys = nonemptyObjectKeys(keys)
		if len(keys) > 0 {
			o, err := s.requireOSS()
			if err != nil {
				return err
			}
			if err = o.DeleteObjects(keys); err != nil {
				return errors.New("OSS 文件删除失败：" + err.Error())
			}
		}
	}
	if err := s.fileRepo.SoftDeleteAll(id); err != nil {
		return err
	}
	return s.assetRepo.SoftDelete(id)
}

// ---------- 文件 ----------

func (s *AssetService) ListFiles(assetID int64) ([]*model.AssetFile, error) {
	list, err := s.fileRepo.ListByAssetID(assetID)
	if err != nil {
		return nil, err
	}
	o := s.oss()
	if o == nil || !o.Ready() {
		return list, nil
	}
	for _, f := range list {
		if u, err := o.URL(f.ObjectKey); err == nil {
			f.URL = u
		}
	}
	return list, nil
}

// UploadFile 上传一个新格式文件（同一素材下后缀唯一）。对象键使用素材所属分类的 ossPrefix。
func (s *AssetService) UploadFile(assetID int64, header *multipart.FileHeader, remark string) (*model.AssetFile, error) {
	remark, err := normalizeFileRemark(remark)
	if err != nil {
		return nil, err
	}
	a, err := s.assetRepo.GetByID(assetID)
	if err != nil {
		return nil, mapNotFound(err, "素材不存在")
	}
	if header.Size > MaxUploadSize {
		return nil, errors.New("文件超过 20MB 限制")
	}
	f, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	suffix := strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
	return s.saveAssetFileBytes(a, data, suffix, remark)
}

// UploadFileFromURL 从远程 http(s) 地址入库：本账号按公共策略直登或重传，外链始终下载重传且不删源。
func (s *AssetService) UploadFileFromURL(assetID int64, sourceURL, remark string) (*model.AssetFile, error) {
	remark, err := normalizeFileRemark(remark)
	if err != nil {
		return nil, err
	}
	a, err := s.assetRepo.GetByID(assetID)
	if err != nil {
		return nil, mapNotFound(err, "素材不存在")
	}
	endpoint, bucket := "", ""
	if s.settings != nil {
		endpoint, bucket = s.settings.OSSEndpointBucket()
	}
	oldKey, ok, parseErr := parseOwnOSSObjectURL(sourceURL, endpoint, bucket)
	if !ok {
		if parseErr != nil {
			return nil, parseErr
		}
		return s.fetchAndSaveAssetFile(a, sourceURL, remark)
	}
	mode, reuploadDeleteSource := "register", false
	if s.settings != nil {
		mode, reuploadDeleteSource, _ = s.settings.CommonPolicy()
	}
	if mode == "reupload" {
		return s.reuploadOwnOSSObject(a, sourceURL, remark, oldKey, reuploadDeleteSource)
	}
	return s.registerOwnOSSObject(a, oldKey, remark)
}

func (s *AssetService) fetchAndSaveAssetFile(a *model.Asset, sourceURL, remark string) (*model.AssetFile, error) {
	data, finalURL, ct, err := fetchRemoteFile(sourceURL)
	if err != nil {
		return nil, err
	}
	suffix, err := resolveRemoteSuffix(finalURL, ct)
	if err != nil {
		return nil, err
	}
	return s.saveAssetFileBytes(a, data, suffix, remark)
}

func (s *AssetService) registerOwnOSSObject(a *model.Asset, key, remark string) (*model.AssetFile, error) {
	o, err := s.requireOSS()
	if err != nil {
		return nil, err
	}
	size, err := o.HeadObjectMeta(key)
	if err != nil {
		if IsOSSObjectNotFound(err) {
			return nil, errors.New("OSS 对象不存在，无法直接登记")
		}
		return nil, err
	}
	suffix := suffixFromURLPath("https://x/" + key)
	if err = s.ensureSuffixAvailable(a.ID, suffix); err != nil {
		return nil, err
	}
	return s.insertAssetFileRecord(a, suffix, remark, key, size, nil, nil)
}

func (s *AssetService) reuploadOwnOSSObject(a *model.Asset, sourceURL, remark, oldKey string, deleteSource bool) (*model.AssetFile, error) {
	af, err := s.fetchAndSaveAssetFile(a, sourceURL, remark)
	if err != nil {
		return nil, err
	}
	if !shouldDeleteReuploadSource(deleteSource, oldKey, af.ObjectKey) {
		return af, nil
	}
	o, ossErr := s.requireOSS()
	if ossErr != nil {
		return af, errors.New("重传成功但删除原对象失败：" + ossErr.Error())
	}
	if delErr := o.DeleteObject(oldKey); delErr != nil {
		return af, errors.New("重传成功但删除原对象失败：" + delErr.Error())
	}
	return af, nil
}

func shouldDeleteReuploadSource(deleteSource bool, oldKey, newKey string) bool {
	return deleteSource && oldKey != "" && newKey != "" && oldKey != newKey
}

func (s *AssetService) ensureSuffixAvailable(assetID int64, suffix string) error {
	if !allowedSuffix[suffix] {
		return errors.New("不支持的格式：" + suffix + "（允许 png/jpg/jpeg/svg/webp/ico/gif）")
	}
	exist, err := s.fileRepo.ExistSuffix(assetID, suffix)
	if err != nil {
		return err
	}
	if exist {
		return errors.New("该素材已存在 ." + suffix + " 文件，请先删除再上传")
	}
	return nil
}

func assetFileName(assetName, suffix, objectKey string) string {
	base := path.Base(objectKey)
	if base == "" || base == "." || base == "/" {
		return assetName + "." + suffix
	}
	return base
}

func (s *AssetService) insertAssetFileRecord(a *model.Asset, suffix, remark, key string, size int64, w, h *int) (*model.AssetFile, error) {
	o, err := s.requireOSS()
	if err != nil {
		return nil, err
	}
	primary := 0
	if has, _ := s.fileRepo.HasAnyFile(a.ID); !has {
		primary = 1
	}
	af := &model.AssetFile{
		AssetID: a.ID, FileName: assetFileName(a.AssetName, suffix, key), Suffix: suffix,
		FileSize: size, Width: w, Height: h, ObjectKey: key,
		IsPrimary: primary, Remark: remark,
	}
	id, err := s.fileRepo.Insert(af)
	if err != nil {
		return nil, err
	}
	af.ID = id
	af.URL, _ = o.URL(key)
	return af, nil
}

func (s *AssetService) saveAssetFileBytes(a *model.Asset, data []byte, suffix, remark string) (*model.AssetFile, error) {
	if err := s.ensureSuffixAvailable(a.ID, suffix); err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxUploadSize {
		return nil, errors.New("文件超过 20MB 限制")
	}
	cat, err := s.categoryRepo.Get(a.CategoryID)
	if err != nil {
		return nil, mapNotFound(err, "分类不存在")
	}
	o, err := s.requireOSS()
	if err != nil {
		return nil, err
	}
	key := o.GenObjectKey(cat.OssPrefix, a.AssetName, suffix)
	contentType := suffixMime[suffix]
	if err = o.PutObject(key, bytes.NewReader(data), int64(len(data)), contentType); err != nil {
		return nil, errors.New("上传 OSS 失败：" + err.Error())
	}
	var w, h *int
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		wv, hv := cfg.Width, cfg.Height
		w, h = &wv, &hv
	}
	return s.insertAssetFileRecord(a, suffix, remark, key, int64(len(data)), w, h)
}

func (s *AssetService) UpdateFileRemark(assetID, fileID int64, remark string) (*model.AssetFile, error) {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil || f.AssetID != assetID {
		return nil, errors.New("文件不存在")
	}
	remark, err = normalizeFileRemark(remark)
	if err != nil {
		return nil, err
	}
	if err := s.fileRepo.UpdateRemark(fileID, remark); err != nil {
		return nil, err
	}
	f.Remark = remark
	o := s.oss()
	if o != nil && o.Ready() {
		if u, err := o.URL(f.ObjectKey); err == nil {
			f.URL = u
		}
	}
	return f, nil
}

func (s *AssetService) SetPrimary(assetID, fileID int64) error {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil || f.AssetID != assetID {
		return errors.New("文件不存在")
	}
	return s.fileRepo.SetPrimary(assetID, fileID)
}

func (s *AssetService) DeleteFile(assetID, fileID int64) error {
	f, err := s.fileRepo.GetByID(fileID)
	if err != nil || f.AssetID != assetID {
		return errors.New("文件不存在")
	}
	if s.deleteRemoveOssEnabled() {
		if key := strings.TrimSpace(f.ObjectKey); key != "" {
			o, err := s.requireOSS()
			if err != nil {
				return err
			}
			if err = o.DeleteObject(key); err != nil {
				return errors.New("OSS 删除失败：" + err.Error())
			}
		}
	}
	if err = s.fileRepo.SoftDelete(fileID); err != nil {
		return err
	}
	// 若删的是主图，自动把下一个文件设为主图
	if f.IsPrimary == 1 {
		if list, err := s.fileRepo.ListByAssetID(assetID); err == nil && len(list) > 0 {
			_ = s.fileRepo.SetPrimary(assetID, list[0].ID)
		}
	}
	return nil
}

func (s *AssetService) deleteRemoveOssEnabled() bool {
	if s == nil || s.settings == nil {
		return true
	}
	_, _, v := s.settings.CommonPolicy()
	return v
}

func nonemptyObjectKeys(keys []string) []string {
	var out []string
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}
