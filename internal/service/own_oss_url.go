package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrOSSObjectNotFound 表示 Head 时对象不存在，上层可映射「OSS 对象不存在，无法直接登记」。
var ErrOSSObjectNotFound = errors.New("oss object not found")

// IsOSSObjectNotFound 判断 err 是否为对象不存在。
func IsOSSObjectNotFound(err error) bool {
	return errors.Is(err, ErrOSSObjectNotFound)
}

var errOwnOSSKeyUnrecognized = errors.New("无法识别为本账号 OSS 对象路径")

// parseOwnOSSObjectURL 判断 rawURL 是否指向本账号 Bucket 并解析 objectKey。
// ok=false 且 err=nil → 非本账号；ok=false 且 err!=nil → 本账号 Host 但无法解析 key；忽略 query/fragment。
func parseOwnOSSObjectURL(rawURL, endpoint, bucket string) (key string, ok bool, err error) {
	rawURL = strings.TrimSpace(rawURL)
	endpoint = normalizeOSSEndpointHost(endpoint)
	bucket = strings.TrimSpace(bucket)
	if rawURL == "" || endpoint == "" || bucket == "" {
		return "", false, nil
	}

	u, parseErr := url.Parse(rawURL)
	if parseErr != nil || u.Host == "" {
		return "", false, nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false, nil
	}

	host := u.Hostname()
	virtualHost := bucket + "." + endpoint
	if strings.EqualFold(host, virtualHost) {
		key, err = objectKeyFromURLPath(u.EscapedPath())
		if err != nil {
			return "", false, errOwnOSSKeyUnrecognized
		}
		return key, true, nil
	}
	if strings.EqualFold(host, endpoint) {
		// Host 等于 endpoint 但 path 不是 /{本 bucket}/… → 同区域他桶等，按外链（ok=false, err=nil）
		rest, matched := stripPathStyleBucket(u.EscapedPath(), bucket)
		if !matched {
			return "", false, nil
		}
		if rest == "" {
			return "", false, errOwnOSSKeyUnrecognized
		}
		key, err = url.PathUnescape(rest)
		if err != nil {
			return "", false, errOwnOSSKeyUnrecognized
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return "", false, errOwnOSSKeyUnrecognized
		}
		return key, true, nil
	}
	return "", false, nil
}

func normalizeOSSEndpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimSuffix(endpoint, "/")
	if i := strings.Index(endpoint, "/"); i >= 0 {
		endpoint = endpoint[:i]
	}
	return endpoint
}

func objectKeyFromURLPath(escapedPath string) (string, error) {
	p := strings.TrimPrefix(escapedPath, "/")
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	key, err := url.PathUnescape(p)
	if err != nil {
		return "", err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("empty key")
	}
	return key, nil
}

func stripPathStyleBucket(path, bucket string) (rest string, matched bool) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "", false
	}
	i := strings.Index(path, "/")
	if i < 0 {
		if strings.EqualFold(path, bucket) {
			return "", true
		}
		return "", false
	}
	seg := path[:i]
	if !strings.EqualFold(seg, bucket) {
		return "", false
	}
	return path[i+1:], true
}