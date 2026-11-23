package service

import (
	"database/sql"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"assethub/internal/config"
	"assethub/internal/repo"
)

// CategoryCounter 提供分类数量。为 nil 时 CategoryCount 视为 0。
// Task 3 的 CategoryRepo 将实现该接口。
type CategoryCounter interface {
	Count() (int, error)
}

// SettingsView 是 GET/PUT /api/settings 的响应（密钥不明文）。
type SettingsView struct {
	Endpoint             string `json:"endpoint"`
	Bucket               string `json:"bucket"`
	Private              bool   `json:"private"`
	SignedExpire         int    `json:"signedExpire"`
	URLPrefix            string `json:"urlPrefix"`
	AccessKeyIDMasked    string `json:"accessKeyIdMasked"`
	AccessKeySecretSet   bool   `json:"accessKeySecretSet"`
	BusinessTokenSet     bool   `json:"businessTokenSet"`
	BusinessTokenMasked  string `json:"businessTokenMasked"`
	OssReady             bool   `json:"ossReady"`
	OssError             string `json:"ossError"`
	CategoryCount        int    `json:"categoryCount"`
	SetupComplete        bool   `json:"setupComplete"`
	OwnOssLinkMode       string `json:"ownOssLinkMode"`
	ReuploadDeleteSource bool   `json:"reuploadDeleteSource"`
	DeleteRemoveOss      bool   `json:"deleteRemoveOss"`
}

// SettingsUpdateReq 是 PUT /api/settings 的请求体。
type SettingsUpdateReq struct {
	Endpoint             string `json:"endpoint"`
	Bucket               string `json:"bucket"`
	AccessKeyID          string `json:"accessKeyId"`
	AccessKeySecret      string `json:"accessKeySecret"`
	URLPrefix            string `json:"urlPrefix"`
	BusinessToken        string `json:"businessToken"`
	Private              bool   `json:"private"`
	SignedExpire         int    `json:"signedExpire"`
	ClearBusinessToken   bool   `json:"clearBusinessToken"`
	OwnOssLinkMode       string `json:"ownOssLinkMode"`
	ReuploadDeleteSource bool   `json:"reuploadDeleteSource"`
	DeleteRemoveOss      bool   `json:"deleteRemoveOss"`
}

// SetupStatus 是 GET /api/setup/status 的响应。
type SetupStatus struct {
	OssReady      bool   `json:"ossReady"`
	OssError      string `json:"ossError"`
	CategoryCount int    `json:"categoryCount"`
	SetupComplete bool   `json:"setupComplete"`
}

// SettingsService 互斥持有可热替换的 OSS 客户端与 businessToken。
type SettingsService struct {
	mu      sync.RWMutex
	repo    *repo.SettingsRepo
	counter CategoryCounter
	row     repo.SettingsRow
	hasRow  bool
	oss     *OSSService
}

// NewSettingsService 读库并尝试创建 OSS。失败不退出：持有 Ready=false 的实例。
// counter 为 nil 时 CategoryCount 恒为 0。无设置行时提示去设置页配置。
func NewSettingsService(r *repo.SettingsRepo, counter CategoryCounter) *SettingsService {
	s := &SettingsService{repo: r, counter: counter}
	if r == nil {
		s.oss = &OSSService{errMsg: "尚未配置 OSS"}
		return s
	}
	row, err := r.Get()
	if err != nil {
		msg := err.Error()
		if errors.Is(err, sql.ErrNoRows) {
			msg = "尚未配置 OSS"
		}
		s.oss = &OSSService{errMsg: msg}
		return s
	}
	s.row = *row
	s.hasRow = true
	s.installOSS(*row)
	return s
}

func (s *SettingsService) installOSS(row repo.SettingsRow) {
	cfg := rowToOSS(row)
	if msg := ossPlaceholderReason(row); msg != "" {
		s.oss = &OSSService{cfg: cfg, errMsg: msg}
		log.Printf("OSS 未就绪：%s", msg)
		return
	}
	svc, err := NewOSSService(cfg)
	if err != nil {
		msg := FormatOSSError(err)
		if svc == nil {
			svc = &OSSService{cfg: cfg, errMsg: msg}
		} else {
			svc.errMsg = msg
		}
		s.oss = svc
		log.Printf("OSS 未就绪：%s", msg)
		return
	}
	// 仅创建客户端不够：占位/错误凭证也可能构造成功，启动时再探一次桶。
	if err := TestOSSConnection(cfg); err != nil {
		svc.ready = false
		svc.bucket = nil
		svc.errMsg = err.Error() // 已是 FormatOSSError 后的中文
		s.oss = svc
		log.Printf("OSS 未就绪：%s", svc.errMsg)
		return
	}
	s.oss = svc
}

// ossPlaceholderReason 识别 yaml 示例占位，避免误报 ossReady=true。
func ossPlaceholderReason(row repo.SettingsRow) string {
	bucket := strings.TrimSpace(strings.ToLower(row.Bucket))
	ak := strings.TrimSpace(row.AccessKeyID)
	sk := strings.TrimSpace(row.AccessKeySecret)
	prefix := strings.TrimSpace(strings.ToLower(row.URLPrefix))
	if bucket == "" || bucket == "your-bucket-name" || strings.Contains(bucket, "your-bucket") {
		return "请先在设置中填写真实的 OSS 配置（当前仍是示例占位）"
	}
	if ak == "" || strings.Contains(strings.ToLower(ak), "xxxx") {
		return "请先在设置中填写真实的 OSS 配置（当前仍是示例占位）"
	}
	if sk == "" || strings.Contains(strings.ToLower(sk), "xxxx") {
		return "请先在设置中填写真实的 OSS 配置（当前仍是示例占位）"
	}
	if strings.Contains(prefix, "your-bucket") {
		return "请先在设置中填写真实的 OSS 配置（当前仍是示例占位）"
	}
	return ""
}

func rowToOSS(row repo.SettingsRow) config.OSSConfig {
	return config.OSSConfig{
		Endpoint:        row.Endpoint,
		Bucket:          row.Bucket,
		AccessKeyID:     row.AccessKeyID,
		AccessKeySecret: row.AccessKeySecret,
		Private:         row.Private,
		SignedExpire:    row.SignedExpire,
		URLPrefix:       row.URLPrefix,
	}
}

// OSS 返回当前 OSS 实例。热更新后需重新获取。
func (s *SettingsService) OSS() *OSSService {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.oss
}

// BusinessToken 返回当前业务 token。空字符串表示不校验。
func (s *SettingsService) BusinessToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.row.BusinessToken
}

// CommonPolicy 返回本账号 OSS 链接策略。无设置行时 register、不删源、删除时移除 OSS。
func (s *SettingsService) CommonPolicy() (mode string, reuploadDeleteSource, deleteRemoveOss bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return policyFromRow(s.row, s.hasRow)
}

// OSSEndpointBucket 返回当前设置中的 Endpoint 与 Bucket（用于本账号 URL 识别）。
func (s *SettingsService) OSSEndpointBucket() (endpoint, bucket string) {
	if s == nil {
		return "", ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.row.Endpoint), strings.TrimSpace(s.row.Bucket)
}

// ossConnectionTester 供 Update 试连；单测可替换以避免真实 OSS。
var ossConnectionTester = TestOSSConnection

// View 返回脱敏后的设置与就绪状态。
func (s *SettingsService) View() SettingsView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.viewLocked()
}

// SetupStatus 返回引导条所需的四字段。
func (s *SettingsService) SetupStatus() SetupStatus {
	v := s.View()
	return SetupStatus{
		OssReady:      v.OssReady,
		OssError:      v.OssError,
		CategoryCount: v.CategoryCount,
		SetupComplete: v.SetupComplete,
	}
}

// Update 合并请求、试连、写库并热替换 OSS。试连或校验失败时不写库、不替换。
func (s *SettingsService) Update(req SettingsUpdateReq) (SettingsView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	old := s.row
	if !s.hasRow && s.repo != nil {
		row, err := s.repo.Get()
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return SettingsView{}, err
		}
		if err == nil {
			old = *row
		}
	}
	merged := mergeSettings(old, req)
	if err := validateSettings(merged); err != nil {
		return SettingsView{}, err
	}
	normalizePolicyFields(&merged)
	ossConnUnchanged := ossConnectionFieldsEqual(old, merged)
	ossCfg := rowToOSS(merged)
	if !ossConnUnchanged {
		if err := ossConnectionTester(ossCfg); err != nil {
			return SettingsView{}, err
		}
	}
	merged.UpdateTime = time.Now().Format(repoTimeLayout)
	if s.repo != nil {
		if err := s.repo.Upsert(merged); err != nil {
			return SettingsView{}, err
		}
	}
	s.row = merged
	s.hasRow = true
	if !ossConnUnchanged {
		s.installOSS(merged)
		if s.oss == nil || !s.oss.Ready() {
			msg := "OSS 未就绪"
			if s.oss != nil && s.oss.LastError() != "" {
				msg = s.oss.LastError()
			}
			return s.viewLocked(), errors.New(msg)
		}
	}
	return s.viewLocked(), nil
}

const repoTimeLayout = "2006-01-02 15:04:05"

// mergeSettings 按设计合并：明文项以请求为准；AK/SK/token 空串保留原值；clearBusinessToken 清空 token。
func mergeSettings(old repo.SettingsRow, req SettingsUpdateReq) repo.SettingsRow {
	merged := old
	merged.Endpoint = strings.TrimSpace(req.Endpoint)
	merged.Bucket = strings.TrimSpace(req.Bucket)
	merged.URLPrefix = strings.TrimSpace(req.URLPrefix)
	merged.Private = req.Private
	merged.SignedExpire = req.SignedExpire
	if ak := strings.TrimSpace(req.AccessKeyID); ak != "" {
		merged.AccessKeyID = ak
	}
	if sk := strings.TrimSpace(req.AccessKeySecret); sk != "" {
		merged.AccessKeySecret = sk
	}
	if req.ClearBusinessToken {
		merged.BusinessToken = ""
	} else if tok := strings.TrimSpace(req.BusinessToken); tok != "" {
		merged.BusinessToken = tok
	}
	merged.OwnOssLinkMode = strings.TrimSpace(req.OwnOssLinkMode)
	merged.ReuploadDeleteSource = req.ReuploadDeleteSource
	merged.DeleteRemoveOss = req.DeleteRemoveOss
	return merged
}

func validateSettings(row repo.SettingsRow) error {
	if row.Endpoint == "" || row.Bucket == "" || row.AccessKeyID == "" || row.AccessKeySecret == "" || row.URLPrefix == "" {
		return errors.New("endpoint、bucket、accessKeyId、accessKeySecret、urlPrefix 不能为空")
	}
	if row.SignedExpire <= 0 {
		return errors.New("signedExpire 必须大于 0")
	}
	mode := strings.TrimSpace(row.OwnOssLinkMode)
	if mode == "" {
		mode = "register"
	}
	if mode != "register" && mode != "reupload" {
		return errors.New("本账号链接处理方式无效")
	}
	return nil
}

func policyFromRow(row repo.SettingsRow, hasRow bool) (mode string, reuploadDeleteSource, deleteRemoveOss bool) {
	if !hasRow {
		return "register", false, true
	}
	mode = strings.TrimSpace(row.OwnOssLinkMode)
	if mode == "" {
		mode = "register"
	}
	return mode, row.ReuploadDeleteSource, row.DeleteRemoveOss
}

func normalizePolicyFields(row *repo.SettingsRow) {
	if strings.TrimSpace(row.OwnOssLinkMode) == "" {
		row.OwnOssLinkMode = "register"
	}
}

func ossConnectionFieldsEqual(a, b repo.SettingsRow) bool {
	return a.Endpoint == b.Endpoint &&
		a.Bucket == b.Bucket &&
		a.AccessKeyID == b.AccessKeyID &&
		a.AccessKeySecret == b.AccessKeySecret &&
		a.URLPrefix == b.URLPrefix &&
		a.Private == b.Private &&
		a.SignedExpire == b.SignedExpire
}

// maskAK 长度大于 8 时保留前 4 与后 4，否则为 ****。空串保持空。
func maskAK(id string) string {
	if id == "" {
		return ""
	}
	if len(id) > 8 {
		return id[:4] + "****" + id[len(id)-4:]
	}
	return "****"
}

func (s *SettingsService) viewLocked() SettingsView {
	n := 0
	if s.counter != nil {
		if c, err := s.counter.Count(); err != nil {
			log.Printf("读取分类数量失败：%v", err)
		} else {
			n = c
		}
	}
	ready := s.oss != nil && s.oss.Ready()
	ossErr := ""
	if !ready && s.oss != nil {
		ossErr = s.oss.LastError()
	}
	mode, reuploadDel, deleteRemoveOss := policyFromRow(s.row, s.hasRow)
	v := SettingsView{
		Endpoint:             s.row.Endpoint,
		Bucket:               s.row.Bucket,
		Private:              s.row.Private,
		SignedExpire:         s.row.SignedExpire,
		URLPrefix:            s.row.URLPrefix,
		AccessKeyIDMasked:    maskAK(s.row.AccessKeyID),
		AccessKeySecretSet:   s.row.AccessKeySecret != "",
		BusinessTokenSet:     s.row.BusinessToken != "",
		OssReady:             ready,
		OssError:             ossErr,
		CategoryCount:        n,
		SetupComplete:        ready && n >= 1,
		OwnOssLinkMode:       mode,
		ReuploadDeleteSource: reuploadDel,
		DeleteRemoveOss:      deleteRemoveOss,
	}
	if v.BusinessTokenSet {
		v.BusinessTokenMasked = "****"
	}
	return v
}
