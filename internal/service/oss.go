package service

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"assethub/internal/config"
)

const ossNotReadyMsg = "请先在设置中配置可用的 OSS"

// OSSService 封装 OSS 上传、删除、URL 生成。未就绪时上传/删除/URL 返回明确错误。
type OSSService struct {
	cfg    config.OSSConfig
	bucket *oss.Bucket
	ready  bool
	errMsg string
}

// NewOSSService 创建客户端。失败时仍返回 Ready=false 的空壳，同时返回 err。
func NewOSSService(cfg config.OSSConfig) (*OSSService, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return &OSSService{cfg: cfg, errMsg: err.Error()}, err
	}
	b, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return &OSSService{cfg: cfg, errMsg: err.Error()}, err
	}
	return &OSSService{cfg: cfg, bucket: b, ready: true}, nil
}

// Ready 表示客户端已创建，可以执行对象操作。
func (s *OSSService) Ready() bool {
	return s != nil && s.ready && s.bucket != nil
}

// LastError 返回未就绪原因。已就绪时为空。
func (s *OSSService) LastError() string {
	if s == nil || s.ready {
		return ""
	}
	return s.errMsg
}

func (s *OSSService) guard() error {
	if !s.Ready() {
		return errors.New(ossNotReadyMsg)
	}
	return nil
}

// GenObjectKey 生成对象路径 {prefix}{assetName}.{suffix}。prefix 末尾自动补 /。
func (s *OSSService) GenObjectKey(prefix, assetName, suffix string) string {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%s%s.%s", prefix, assetName, suffix)
}

// TestOSSConnection 用合并后的配置探测 Bucket。SDK 无 HeadBucket，使用 GetBucketInfo；失败返回可读中文错误。
func TestOSSConnection(cfg config.OSSConfig) error {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return errors.New(FormatOSSError(err))
	}
	b, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return errors.New(FormatOSSError(err))
	}
	_, err = b.Client.GetBucketInfo(cfg.Bucket)
	if err != nil {
		return errors.New(FormatOSSError(err))
	}
	return nil
}

var ossErrorCodeRe = regexp.MustCompile(`ErrorCode=([A-Za-z0-9_-]+)`)

// FormatOSSError 将阿里云 SDK 原始错误转成面向用户的中文说明。
func FormatOSSError(err error) string {
	if err == nil {
		return ""
	}
	code := ""
	var se oss.ServiceError
	if errors.As(err, &se) {
		code = se.Code
	}
	if code == "" {
		if m := ossErrorCodeRe.FindStringSubmatch(err.Error()); len(m) == 2 {
			code = m[1]
		}
	}
	switch code {
	case "InvalidAccessKeyId":
		return "AccessKey ID 无效或不存在。请到 RAM 控制台核对 ID，或在本页重新填写 AccessKey ID 与 Secret（当前若留空会沿用库里的旧值，示例占位密钥无法通过校验）。"
	case "SignatureDoesNotMatch":
		return "AccessKey Secret 不正确，或与 AccessKey ID 不匹配。请重新填写完整的 ID 与 Secret 后保存。"
	case "NoSuchBucket":
		return "Bucket 不存在，或与 Endpoint 地域不一致。请核对 Bucket 名称和 Endpoint（如 oss-cn-beijing.aliyuncs.com）。"
	case "AccessDenied":
		return "当前 AccessKey 无权访问该 Bucket。请给 RAM 子账号授予该 Bucket 的读写/读取权限（试连需要 GetBucketInfo）。"
	case "RequestTimeTooSkewed":
		return "本机时间与阿里云服务器偏差过大，请校准系统时间后重试。"
	case "InvalidBucketName":
		return "Bucket 名称格式不正确，请核对控制台中的名称。"
	case "InvalidEndpoint", "InvalidArgument":
		return "Endpoint 或参数不正确。Endpoint 不要带 https://，例如 oss-cn-beijing.aliyuncs.com。"
	}
	raw := err.Error()
	if strings.Contains(strings.ToLower(raw), "timeout") || strings.Contains(strings.ToLower(raw), "deadline") {
		return "连接 OSS 超时，请检查网络，以及 Endpoint 是否可从本机访问（不要用内网 -internal 节点）。"
	}
	if strings.Contains(strings.ToLower(raw), "no such host") || strings.Contains(strings.ToLower(raw), "lookup") {
		return "无法解析 Endpoint，请检查拼写（不要带 https://），并确认本机网络正常。"
	}
	if code != "" {
		return fmt.Sprintf("OSS 校验失败（%s）。请核对 Endpoint、Bucket、AccessKey 后重试。", code)
	}
	return "OSS 校验失败：" + truncateErr(raw, 160)
}

func truncateErr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// PutObject 上传文件到 OSS。r 需为可 Seek 的 reader（本服务使用 bytes.Reader）。
func (s *OSSService) PutObject(key string, r io.ReadSeeker, size int64, contentType string) error {
	if err := s.guard(); err != nil {
		return err
	}
	opts := []oss.Option{
		oss.ContentType(contentType),
		oss.ContentLength(size),
	}
	return s.bucket.PutObject(key, r, opts...)
}

// DeleteObject 删除单个对象。
func (s *OSSService) DeleteObject(key string) error {
	if key == "" {
		return nil
	}
	if err := s.guard(); err != nil {
		return err
	}
	return s.bucket.DeleteObject(key)
}

// DeleteObjects 批量删除（逐个调用，keys 一般只有几个）。
func (s *OSSService) DeleteObjects(keys []string) error {
	var firstErr error
	for _, k := range keys {
		if err := s.DeleteObject(k); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// URL 生成访问链接：公共读直接拼前缀；私有 Bucket 生成带时效签名 URL。
func (s *OSSService) URL(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	if err := s.guard(); err != nil {
		return "", err
	}
	if !s.cfg.Private {
		return strings.TrimRight(s.cfg.URLPrefix, "/") + "/" + key, nil
	}
	return s.bucket.SignURL(key, oss.HTTPGet, int64(s.cfg.SignedExpire))
}
