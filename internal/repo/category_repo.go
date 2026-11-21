package repo

import (
	"database/sql"
	"errors"
	"time"

	"assethub/internal/model"
)

func repoNow() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// CategoryRepo 读写分类。Count 满足 service.CategoryCounter。
type CategoryRepo struct {
	db *sql.DB
}

func NewCategoryRepo(db *sql.DB) *CategoryRepo {
	return &CategoryRepo{db: db}
}

// List 按 sort、id 升序返回全部分类。无记录时为空切片。
func (r *CategoryRepo) List() ([]model.Category, error) {
	rows, err := r.db.Query(`
SELECT id, name, oss_prefix, sort, create_time, update_time
FROM category ORDER BY sort ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]model.Category, 0)
	for rows.Next() {
		var c model.Category
		if err = rows.Scan(&c.ID, &c.Name, &c.OssPrefix, &c.Sort, &c.CreateTime, &c.UpdateTime); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// Get 按 id 读取。不存在时返回 sql.ErrNoRows。
func (r *CategoryRepo) Get(id int64) (*model.Category, error) {
	var c model.Category
	err := r.db.QueryRow(`
SELECT id, name, oss_prefix, sort, create_time, update_time
FROM category WHERE id = ?`, id).Scan(
		&c.ID, &c.Name, &c.OssPrefix, &c.Sort, &c.CreateTime, &c.UpdateTime)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Insert 写入分类并回填创建/更新时间，返回新 id。
func (r *CategoryRepo) Insert(c *model.Category) (int64, error) {
	t := repoNow()
	res, err := r.db.Exec(`
INSERT INTO category(name, oss_prefix, sort, create_time, update_time)
VALUES (?,?,?,?,?)`, c.Name, c.OssPrefix, c.Sort, t, t)
	if err != nil {
		return 0, err
	}
	c.CreateTime = t
	c.UpdateTime = t
	return res.LastInsertId()
}

// Update 按 id 更新名称、前缀与排序。不存在时返回 sql.ErrNoRows。
func (r *CategoryRepo) Update(c *model.Category) error {
	var exists int
	err := r.db.QueryRow(`SELECT 1 FROM category WHERE id = ?`, c.ID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	c.UpdateTime = repoNow()
	_, err = r.db.Exec(`
UPDATE category SET name = ?, oss_prefix = ?, sort = ?, update_time = ?
WHERE id = ?`, c.Name, c.OssPrefix, c.Sort, c.UpdateTime, c.ID)
	return err
}

// Delete 删除分类。不存在时返回 sql.ErrNoRows。仍被素材引用时由外键拒绝。
func (r *CategoryRepo) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM category WHERE id = ?`, id)
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
	return nil
}

// Count 返回分类总数。
func (r *CategoryRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM category`).Scan(&n)
	return n, err
}

// CountAssets 统计 category_id 指向该分类的素材行，含 del_flag=1。
// 软删行仍持有外键，删除分类前必须计入，否则会落到外键错误而不是业务错误。
func (r *CategoryRepo) CountAssets(categoryID int64) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM asset WHERE category_id = ?`, categoryID).Scan(&n)
	return n, err
}
