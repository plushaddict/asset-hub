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
    create_time     TEXT NOT NULL DEFAULT '',
    del_flag        INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (asset_id) REFERENCES asset(id)
);
CREATE INDEX IF NOT EXISTS idx_af_asset ON asset_file(asset_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_af_asset_suffix ON asset_file(asset_id, suffix) WHERE del_flag = 0;
`
	_, err := db.Exec(schema)
	return err
}