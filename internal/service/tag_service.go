package service

import (
	"errors"
	"strings"

	"assethub/internal/model"
	"assethub/internal/repo"
)

// TagService 校验标签名称，并把唯一冲突转成中文错误。
type TagService struct {
	repo *repo.TagRepo
}

func NewTagService(r *repo.TagRepo) *TagService {
	return &TagService{repo: r}
}

func (s *TagService) List() ([]model.Tag, error) {
	return s.repo.List()
}

func (s *TagService) Create(name string) (*model.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("标签名称不能为空")
	}
	t := &model.Tag{Name: name}
	id, err := s.repo.Insert(t)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errors.New("标签名称已存在")
		}
		return nil, err
	}
	t.ID = id
	return t, nil
}

func (s *TagService) Update(id int64, name string) (*model.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("标签名称不能为空")
	}
	t := &model.Tag{ID: id, Name: name}
	if err := s.repo.Update(t); err != nil {
		if isUniqueViolation(err) {
			return nil, errors.New("标签名称已存在")
		}
		return nil, mapNotFound(err, "标签不存在")
	}
	return s.repo.Get(id)
}

// Delete 先清关联再删标签（由 repo 在同一事务内完成）。
func (s *TagService) Delete(id int64) error {
	return mapNotFound(s.repo.Delete(id), "标签不存在")
}
