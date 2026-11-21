package repo

import (
	"database/sql"
	"errors"
	"time"

	"assethub/internal/config"
)

const settingsTimeLayout = "2006-01-02 15:04:05"

// SettingsRow 是 system_settings 的单行（id 恒为 1）。
type SettingsRow struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	URLPrefix       string
	BusinessToken   string
	Private         bool
	SignedExpire    int
	UpdateTime      string
}

// SettingsRepo 读写系统设置单行。
type SettingsRepo struct {
	db *sql.DB
}

func NewSettingsRepo(db *sql.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

// Get 读取 id=1。无行时返回 sql.ErrNoRows。
func (r *SettingsRepo) Get() (*SettingsRow, error) {
	var row SettingsRow
	var priv int
	err := r.db.QueryRow(`
SELECT endpoint, bucket, access_key_id, access_key_secret, private,
       signed_expire, url_prefix, business_token, update_time
FROM system_settings WHERE id = 1`).Scan(
		&row.Endpoint, &row.Bucket, &row.AccessKeyID, &row.AccessKeySecret, &priv,
		&row.SignedExpire, &row.URLPrefix, &row.BusinessToken, &row.UpdateTime,
	)
	if err != nil {
		return nil, err
	}
	row.Private = priv != 0
	return &row, nil
}

// Upsert 写入 id=1。UpdateTime 为空时使用当前时间。
func (r *SettingsRepo) Upsert(row SettingsRow) error {
	if row.UpdateTime == "" {
		row.UpdateTime = time.Now().Format(settingsTimeLayout)
	}
	priv := 0
	if row.Private {
		priv = 1
	}
	_, err := r.db.Exec(`
INSERT INTO system_settings (
    id, endpoint, bucket, access_key_id, access_key_secret,
    private, signed_expire, url_prefix, business_token, update_time
) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    endpoint = excluded.endpoint,
    bucket = excluded.bucket,
    access_key_id = excluded.access_key_id,
    access_key_secret = excluded.access_key_secret,
    private = excluded.private,
    signed_expire = excluded.signed_expire,
    url_prefix = excluded.url_prefix,
    business_token = excluded.business_token,
    update_time = excluded.update_time`,
		row.Endpoint, row.Bucket, row.AccessKeyID, row.AccessKeySecret,
		priv, row.SignedExpire, row.URLPrefix, row.BusinessToken, row.UpdateTime,
	)
	return err
}

// EnsureSeeded 在无行时用 yaml 的 oss 与 server.business_token 插入。已有行则不覆盖。
func (r *SettingsRepo) EnsureSeeded(from *config.Config) error {
	if from == nil {
		return errors.New("config is nil")
	}
	_, err := r.Get()
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	expire := from.OSS.SignedExpire
	if expire <= 0 {
		expire = 3600
	}
	return r.Upsert(SettingsRow{
		Endpoint:        from.OSS.Endpoint,
		Bucket:          from.OSS.Bucket,
		AccessKeyID:     from.OSS.AccessKeyID,
		AccessKeySecret: from.OSS.AccessKeySecret,
		Private:         from.OSS.Private,
		SignedExpire:    expire,
		URLPrefix:       from.OSS.URLPrefix,
		BusinessToken:   from.Server.BusinessToken,
	})
}
