package repo

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Open 打开（或创建）SQLite 数据库文件，纯 Go 驱动，无需 CGO
func Open(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite 单写者，限制单连接避免锁冲突
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate 初始化表结构
func Migrate(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS system_settings (
    id                INTEGER PRIMARY KEY,
    endpoint          TEXT NOT NULL,
    bucket            TEXT NOT NULL,
    access_key_id     TEXT NOT NULL,
    access_key_secret TEXT NOT NULL,
    private           INTEGER NOT NULL,
    signed_expire     INTEGER NOT NULL,
    url_prefix        TEXT NOT NULL,
    business_token    TEXT NOT NULL DEFAULT '',
    own_oss_link_mode TEXT NOT NULL DEFAULT 'register',
    reupload_delete_source INTEGER NOT NULL DEFAULT 0,
    delete_remove_oss INTEGER NOT NULL DEFAULT 1,
    update_time       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS category (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT UNIQUE NOT NULL,
    oss_prefix  TEXT NOT NULL,
    sort        INTEGER NOT NULL DEFAULT 0,
    create_time TEXT NOT NULL DEFAULT '',
    update_time TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tag (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT UNIQUE NOT NULL,
    create_time TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS asset (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_name  TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    category_id INTEGER NOT NULL,
    remark      TEXT NOT NULL DEFAULT '',
    sort        INTEGER NOT NULL DEFAULT 0,
    create_by   TEXT NOT NULL DEFAULT '',
    create_time TEXT NOT NULL DEFAULT '',
    update_time TEXT NOT NULL DEFAULT '',
    del_flag    INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (category_id) REFERENCES category(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_asset_name ON asset(asset_name) WHERE del_flag = 0;

CREATE TABLE IF NOT EXISTS asset_tag (
    asset_id INTEGER NOT NULL,
    tag_id   INTEGER NOT NULL,
    PRIMARY KEY (asset_id, tag_id),
    FOREIGN KEY (asset_id) REFERENCES asset(id),
    FOREIGN KEY (tag_id) REFERENCES tag(id)
);

CREATE TABLE IF NOT EXISTS asset_file (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_id        INTEGER NOT NULL,
    file_name       TEXT NOT NULL,
    suffix          TEXT NOT NULL,
    file_size       INTEGER NOT NULL DEFAULT 0,
    width           INTEGER,
    height          INTEGER,
    oss_object_key  TEXT NOT NULL,
    is_primary      INTEGER NOT NULL DEFAULT 0,
    remark          TEXT NOT NULL DEFAULT '',
    create_time     TEXT NOT NULL DEFAULT '',
    del_flag        INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (asset_id) REFERENCES asset(id)
);
CREATE INDEX IF NOT EXISTS idx_af_asset ON asset_file(asset_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_af_asset_suffix ON asset_file(asset_id, suffix) WHERE del_flag = 0;
`
	_, err := db.Exec(schema)
	if err != nil {
		return err
	}
	if err = ensureAssetDisplayNameColumn(db); err != nil {
		return err
	}
	if err = ensureAssetFileRemarkColumn(db); err != nil {
		return err
	}
	return ensureSettingsPolicyColumns(db)
}

func ensureAssetDisplayNameColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(asset)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	has := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "display_name" {
			has = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !has {
		if _, err = db.Exec(`ALTER TABLE asset ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	_, err = db.Exec(`UPDATE asset SET display_name = asset_name WHERE display_name = '' OR display_name IS NULL`)
	return err
}

func ensureAssetFileRemarkColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(asset_file)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	has := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "remark" {
			has = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE asset_file ADD COLUMN remark TEXT NOT NULL DEFAULT ''`)
	return err
}

func ensureSettingsPolicyColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(system_settings)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	has := map[string]bool{
		"own_oss_link_mode":      false,
		"reupload_delete_source": false,
		"delete_remove_oss":      false,
	}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if _, ok := has[name]; ok {
			has[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	alters := []struct {
		name string
		ddl  string
	}{
		{"own_oss_link_mode", `ALTER TABLE system_settings ADD COLUMN own_oss_link_mode TEXT NOT NULL DEFAULT 'register'`},
		{"reupload_delete_source", `ALTER TABLE system_settings ADD COLUMN reupload_delete_source INTEGER NOT NULL DEFAULT 0`},
		{"delete_remove_oss", `ALTER TABLE system_settings ADD COLUMN delete_remove_oss INTEGER NOT NULL DEFAULT 1`},
	}
	for _, a := range alters {
		if has[a.name] {
			continue
		}
		if _, err = db.Exec(a.ddl); err != nil {
			return err
		}
	}
	return nil
}