package service

import (
	"database/sql"
	"errors"
	"strings"

	"assethub/internal/model"
	"assethub/internal/repo"
)

// 编译期确认 CategoryRepo 可交给 SettingsService 当分类计数。
var _ CategoryCounter = (*repo.CategoryRepo)(nil)

// CategoryService 校验分类名称与 OSS 前缀，并把唯一冲突转成中文错误。
type CategoryService struct {
	repo *repo.CategoryRepo
}

func NewCategoryService(r *repo.CategoryRepo) *CategoryService {
	return &CategoryService{repo: r}
}

// NormalizePrefix 去掉首尾空白。空白返回错误；不以 / 结尾则补上。
func NormalizePrefix(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("ossPrefix 不能为空")
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p, nil
}

func (s *CategoryService) List() ([]model.Category, error) {
	return s.repo.List()
}

func (s *CategoryService) Get(id int64) (*model.Category, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		return nil, mapNotFound(err, "分类不存在")
	}
	return c, nil
}

func (s *CategoryService) Create(name, ossPrefix string, sort int) (*model.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("分类名称不能为空")
	}
	prefix, err := NormalizePrefix(ossPrefix)
	if err != nil {
		return nil, err
	}
	c := &model.Category{Name: name, OssPrefix: prefix, Sort: sort}
	id, err := s.repo.Insert(c)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errors.New("分类名称已存在")
		}
		return nil, err
	}
	c.ID = id
	return c, nil
}

func (s *CategoryService) Update(id int64, name, ossPrefix string, sort int) (*model.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("分类名称不能为空")
	}
	prefix, err := NormalizePrefix(ossPrefix)
	if err != nil {
		return nil, err
	}
	c := &model.Category{ID: id, Name: name, OssPrefix: prefix, Sort: sort}
	if err = s.repo.Update(c); err != nil {
		if isUniqueViolation(err) {
			return nil, errors.New("分类名称已存在")
		}
		return nil, mapNotFound(err, "分类不存在")
	}
	return s.repo.Get(id)
}

// Delete 在分类下仍有素材时拒绝。CountAssets 含软删行，避免外键错误盖过业务提示。
func (s *CategoryService) Delete(id int64) error {
	n, err := s.repo.CountAssets(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return errors.New("该分类下仍有素材，无法删除")
	}
	err = s.repo.Delete(id)
	return mapNotFound(err, "分类不存在")
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func mapNotFound(err error, msg string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New(msg)
	}
	return err
}
