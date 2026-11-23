package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const remoteFetchTimeout = 15 * time.Second
const remoteMaxRedirects = 5

func defaultRemoteHTTPClient() *http.Client {
	return &http.Client{
		Timeout: remoteFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= remoteMaxRedirects {
				return errors.New("重定向次数过多")
			}
			return nil
		},
	}
}

var remoteHTTPClient = defaultRemoteHTTPClient()

func suffixFromURLPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return ""
	}
	base := path.Base(u.Path)
	i := strings.LastIndex(base, ".")
	if i < 0 || i == len(base)-1 {
		return ""
	}
	suf := strings.ToLower(base[i+1:])
	if !allowedSuffix[suf] {
		return ""
	}
	return suf
}

func suffixFromContentType(ct string) string {
	ct = strings.TrimSpace(strings.Split(ct, ";")[0])
	ct = strings.ToLower(ct)
	switch ct {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/svg+xml":
		return "svg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "image/x-icon", "image/vnd.microsoft.icon":
		return "ico"
	default:
		return ""
	}
}

func resolveRemoteSuffix(finalURL, contentType string) (string, error) {
	if s := suffixFromURLPath(finalURL); s != "" {
		return s, nil
	}
	if s := suffixFromContentType(contentType); s != "" {
		return s, nil
	}
	return "", errors.New("无法识别远程文件格式（请使用带 png/jpg/jpeg/svg/webp/ico/gif 扩展名的链接，或确保响应 Content-Type 为图片类型）")
}

func fetchRemoteFile(rawURL string) ([]byte, string, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, "", "", errors.New("远程 URL 无效")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", "", errors.New("远程 URL 仅支持 http/https")
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", errors.New("远程文件下载失败：" + err.Error())
	}
	resp, err := remoteHTTPClient.Do(req)
	if err != nil {
		return nil, "", "", errors.New("远程文件下载失败：" + err.Error())
	}
	defer resp.Body.Close()
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("远程文件下载失败：HTTP %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, int64(MaxUploadSize)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", "", errors.New("远程文件下载失败：" + err.Error())
	}
	if len(data) > MaxUploadSize {
		return nil, "", "", errors.New("文件超过 20MB 限制")
	}
	ct := resp.Header.Get("Content-Type")
	return data, finalURL, ct, nil
}