package repo

import (
	"database/sql"

	"assethub/internal/model"
)

type AssetFileRepo struct {
	db *sql.DB
}

func NewAssetFileRepo(db *sql.DB) *AssetFileRepo {
	return &AssetFileRepo{db: db}
}

func (r *AssetFileRepo) Insert(f *model.AssetFile) (int64, error) {
	f.CreateTime = repoNow()
	res, err := r.db.Exec(
		`INSERT INTO asset_file(asset_id, file_name, suffix, file_size, width, height, oss_object_key, is_primary, create_time)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		f.AssetID, f.FileName, f.Suffix, f.FileSize, f.Width, f.Height, f.ObjectKey, f.IsPrimary, f.CreateTime)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *AssetFileRepo) ListByAssetID(assetID int64) ([]*model.AssetFile, error) {
	rows, err := r.db.Query(
		`SELECT id, asset_id, file_name, suffix, file_size, width, height, oss_object_key, is_primary, create_time
		 FROM asset_file WHERE asset_id = ? AND del_flag = 0
		 ORDER BY is_primary DESC, id ASC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.AssetFile
	for rows.Next() {
		var f model.AssetFile
		if err = rows.Scan(&f.ID, &f.AssetID, &f.FileName, &f.Suffix, &f.FileSize,
			&f.Width, &f.Height, &f.ObjectKey, &f.IsPrimary, &f.CreateTime); err != nil {
			return nil, err
		}
		list = append(list, &f)
	}
	return list, rows.Err()
}

func (r *AssetFileRepo) GetByID(id int64) (*model.AssetFile, error) {
	var f model.AssetFile
	err := r.db.QueryRow(
		`SELECT id, asset_id, file_name, suffix, file_size, width, height, oss_object_key, is_primary, create_time
		 FROM asset_file WHERE id = ? AND del_flag = 0`, id).
		Scan(&f.ID, &f.AssetID, &f.FileName, &f.Suffix, &f.FileSize,
			&f.Width, &f.Height, &f.ObjectKey, &f.IsPrimary, &f.CreateTime)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *AssetFileRepo) ExistSuffix(assetID int64, suffix string) (bool, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM asset_file WHERE asset_id = ? AND suffix = ? AND del_flag = 0`,
		assetID, suffix).Scan(&n)
	return n > 0, err
}

func (r *AssetFileRepo) HasAnyFile(assetID int64) (bool, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM asset_file WHERE asset_id = ? AND del_flag = 0`, assetID).Scan(&n)
	return n > 0, err
}

func (r *AssetFileRepo) SetPrimary(assetID, fileID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE asset_file SET is_primary = 0 WHERE asset_id = ? AND del_flag = 0`, assetID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = tx.Exec(`UPDATE asset_file SET is_primary = 1 WHERE id = ? AND del_flag = 0`, fileID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *AssetFileRepo) PrimaryFile(assetID int64) (*model.AssetFile, error) {
	var f model.AssetFile
	err := r.db.QueryRow(
		`SELECT id, asset_id, file_name, suffix, file_size, width, height, oss_object_key, is_primary, create_time
		 FROM asset_file WHERE asset_id = ? AND del_flag = 0
		 ORDER BY is_primary DESC, id ASC LIMIT 1`, assetID).
		Scan(&f.ID, &f.AssetID, &f.FileName, &f.Suffix, &f.FileSize,
			&f.Width, &f.Height, &f.ObjectKey, &f.IsPrimary, &f.CreateTime)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *AssetFileRepo) SoftDelete(id int64) error {
	_, err := r.db.Exec(`UPDATE asset_file SET del_flag = 1 WHERE id = ?`, id)
	return err
}

func (r *AssetFileRepo) SoftDeleteAll(assetID int64) error {
	_, err := r.db.Exec(`UPDATE asset_file SET del_flag = 1 WHERE asset_id = ?`, assetID)
	return err
}

func (r *AssetFileRepo) ObjectKeys(assetID int64) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT oss_object_key FROM asset_file WHERE asset_id = ? AND del_flag = 0`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err = rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
