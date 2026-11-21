package repo

import (
	"database/sql"
	"errors"
	"strings"

	"assethub/internal/model"
)

// TagRepo 读写标签字典与素材关联。
type TagRepo struct {
	db *sql.DB
}

func NewTagRepo(db *sql.DB) *TagRepo {
	return &TagRepo{db: db}
}

// List 按 id 升序返回全部标签。无记录时为空切片。
func (r *TagRepo) List() ([]model.Tag, error) {
	rows, err := r.db.Query(`SELECT id, name, create_time FROM tag ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]model.Tag, 0)
	for rows.Next() {
		var t model.Tag
		if err = rows.Scan(&t.ID, &t.Name, &t.CreateTime); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// Get 按 id 读取。不存在时返回 sql.ErrNoRows。
func (r *TagRepo) Get(id int64) (*model.Tag, error) {
	var t model.Tag
	err := r.db.QueryRow(`SELECT id, name, create_time FROM tag WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.CreateTime)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Insert 写入标签并回填创建时间，返回新 id。
func (r *TagRepo) Insert(t *model.Tag) (int64, error) {
	t.CreateTime = repoNow()
	res, err := r.db.Exec(`INSERT INTO tag(name, create_time) VALUES (?, ?)`, t.Name, t.CreateTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update 按 id 更新名称。不存在时返回 sql.ErrNoRows。
func (r *TagRepo) Update(t *model.Tag) error {
	var exists int
	err := r.db.QueryRow(`SELECT 1 FROM tag WHERE id = ?`, t.ID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`UPDATE tag SET name = ? WHERE id = ?`, t.Name, t.ID)
	return err
}

// Delete 先清掉该标签的素材关联，再删标签。不存在时返回 sql.ErrNoRows。
func (r *TagRepo) Delete(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM asset_tag WHERE tag_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM tag WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

// UpsertByName 按名称取 id；没有则插入并返回新 id。
func (r *TagRepo) UpsertByName(name string) (int64, error) {
	var id int64
	err := r.db.QueryRow(`SELECT id FROM tag WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	res, err := r.db.Exec(`INSERT INTO tag(name, create_time) VALUES (?, ?)`, name, repoNow())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			if err = r.db.QueryRow(`SELECT id FROM tag WHERE name = ?`, name).Scan(&id); err != nil {
				return 0, err
			}
			return id, nil
		}
		return 0, err
	}
	return res.LastInsertId()
}

// ReplaceAssetTags 用给定标签全量替换素材关联。tagIDs 为空则清空。重复 id 只插一次。
func (r *TagRepo) ReplaceAssetTags(assetID int64, tagIDs []int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM asset_tag WHERE asset_id = ?`, assetID); err != nil {
		return err
	}
	if len(tagIDs) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO asset_tag(asset_id, tag_id) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		seen := make(map[int64]struct{}, len(tagIDs))
		for _, tagID := range tagIDs {
			if _, ok := seen[tagID]; ok {
				continue
			}
			seen[tagID] = struct{}{}
			if _, err = stmt.Exec(assetID, tagID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// ListByAssetID 返回素材当前关联的标签，按标签 id 升序。
func (r *TagRepo) ListByAssetID(assetID int64) ([]model.Tag, error) {
	rows, err := r.db.Query(`
SELECT t.id, t.name, t.create_time
FROM tag t
JOIN asset_tag at ON at.tag_id = t.id
WHERE at.asset_id = ?
ORDER BY t.id ASC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]model.Tag, 0)
	for rows.Next() {
		var t model.Tag
		if err = rows.Scan(&t.ID, &t.Name, &t.CreateTime); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}
