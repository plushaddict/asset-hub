package repo

import (
	"database/sql"
	"strings"

	"assethub/internal/model"
)

type AssetRepo struct {
	db *sql.DB
}

func NewAssetRepo(db *sql.DB) *AssetRepo {
	return &AssetRepo{db: db}
}

func (r *AssetRepo) Insert(a *model.Asset) (int64, error) {
	t := repoNow()
	res, err := r.db.Exec(
		`INSERT INTO asset(asset_name, category_id, remark, sort, create_by, create_time, update_time)
		 VALUES (?,?,?,?,?,?,?)`,
		a.AssetName, a.CategoryID, a.Remark, a.Sort, a.CreateBy, t, t)
	if err != nil {
		return 0, err
	}
	a.CreateTime = t
	a.UpdateTime = t
	return res.LastInsertId()
}

func (r *AssetRepo) GetByID(id int64) (*model.Asset, error) {
	return r.getOne("a.id = ?", id)
}

func (r *AssetRepo) GetByName(name string) (*model.Asset, error) {
	return r.getOne("a.asset_name = ?", name)
}

func (r *AssetRepo) getOne(where string, arg any) (*model.Asset, error) {
	var a model.Asset
	err := r.db.QueryRow(`
SELECT a.id, a.asset_name, a.category_id, c.name, a.remark, a.sort, a.create_by, a.create_time, a.update_time
FROM asset a
JOIN category c ON c.id = a.category_id
WHERE a.del_flag = 0 AND `+where, arg).Scan(
		&a.ID, &a.AssetName, &a.CategoryID, &a.CategoryName, &a.Remark, &a.Sort,
		&a.CreateBy, &a.CreateTime, &a.UpdateTime)
	if err != nil {
		return nil, err
	}
	if err = r.fillTags([]*model.Asset{&a}); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AssetRepo) Update(a *model.Asset) error {
	_, err := r.db.Exec(
		`UPDATE asset SET asset_name = ?, category_id = ?, remark = ?, sort = ?, update_time = ?
		 WHERE id = ? AND del_flag = 0`,
		a.AssetName, a.CategoryID, a.Remark, a.Sort, repoNow(), a.ID)
	return err
}

func (r *AssetRepo) SoftDelete(id int64) error {
	_, err := r.db.Exec(`UPDATE asset SET del_flag = 1, update_time = ? WHERE id = ?`, repoNow(), id)
	return err
}

// NormalizeSort 将 API 排序参数映射为 SQL 列与方向；非法值按字段独立回退默认。
func NormalizeSort(sortBy, sortOrder string) (col, dir string) {
	switch sortBy {
	case "updateTime":
		col = "a.update_time"
	case "assetName":
		col = "a.asset_name"
	case "sort":
		col = "a.sort"
	default: // createTime 及非法值
		col = "a.create_time"
	}
	if sortOrder == "asc" {
		dir = "ASC"
	} else {
		dir = "DESC"
	}
	return col, dir
}

// PageList 按关键字、分类、标签 AND 分页。keyword 只匹配名称和备注。
// 每个 tagID 追加一条 EXISTS。结果带 CategoryName，并填上 Tags（无标签为空切片）。
func (r *AssetRepo) PageList(keyword string, categoryID int64, tagIDs []int64, page, size int, sortBy, sortOrder string) ([]*model.Asset, int, error) {
	where := "a.del_flag = 0"
	args := make([]any, 0, 4+len(tagIDs))
	if keyword != "" {
		where += " AND (a.asset_name LIKE ? OR a.remark LIKE ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	if categoryID > 0 {
		where += " AND a.category_id = ?"
		args = append(args, categoryID)
	}
	for _, tagID := range tagIDs {
		where += " AND EXISTS (SELECT 1 FROM asset_tag at WHERE at.asset_id = a.id AND at.tag_id = ?)"
		args = append(args, tagID)
	}

	from := " FROM asset a JOIN category c ON c.id = a.category_id WHERE " + where

	var total int
	if err := r.db.QueryRow("SELECT COUNT(*)"+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	col, dir := NormalizeSort(sortBy, sortOrder)
	q := `SELECT a.id, a.asset_name, a.category_id, c.name, a.remark, a.sort, a.create_by,
	             a.create_time, a.update_time,
	             (SELECT COUNT(*) FROM asset_file f WHERE f.asset_id = a.id AND f.del_flag = 0)` +
		from + ` ORDER BY ` + col + ` ` + dir + `, a.id DESC LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := r.db.Query(q, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*model.Asset
	for rows.Next() {
		var a model.Asset
		if err = rows.Scan(&a.ID, &a.AssetName, &a.CategoryID, &a.CategoryName, &a.Remark, &a.Sort,
			&a.CreateBy, &a.CreateTime, &a.UpdateTime, &a.FileCount); err != nil {
			return nil, 0, err
		}
		list = append(list, &a)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	if err = r.fillTags(list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// fillTags 按标签 id 升序填每个素材的 Tags。没有关联时为非 nil 空切片。
func (r *AssetRepo) fillTags(list []*model.Asset) error {
	if len(list) == 0 {
		return nil
	}
	args := make([]any, len(list))
	holders := make([]string, len(list))
	byID := make(map[int64]*model.Asset, len(list))
	for i, a := range list {
		args[i] = a.ID
		holders[i] = "?"
		a.Tags = []model.Tag{}
		byID[a.ID] = a
	}
	rows, err := r.db.Query(`
SELECT at.asset_id, t.id, t.name, t.create_time
FROM asset_tag at
JOIN tag t ON t.id = at.tag_id
WHERE at.asset_id IN (`+strings.Join(holders, ",")+`)
ORDER BY t.id ASC`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var assetID int64
		var tag model.Tag
		if err = rows.Scan(&assetID, &tag.ID, &tag.Name, &tag.CreateTime); err != nil {
			return err
		}
		if a := byID[assetID]; a != nil {
			a.Tags = append(a.Tags, tag)
		}
	}
	return rows.Err()
}
