package service

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

var nonSlugCharRe = regexp.MustCompile(`[^a-z0-9]+`)

func SlugFromDisplayName(display string) string {
	args := pinyin.NewArgs()
	args.Style = pinyin.Normal
	var b strings.Builder
	for _, r := range display {
		switch {
		case unicode.Is(unicode.Han, r):
			syllables := pinyin.Pinyin(string(r), args)
			if len(syllables) == 0 || len(syllables[0]) == 0 {
				b.WriteByte('_')
				continue
			}
			b.WriteString(strings.ToLower(syllables[0][0]))
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return finalizeAssetSlug(b.String())
}

func finalizeAssetSlug(raw string) string {
	s := strings.ToLower(raw)
	s = nonSlugCharRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	if s == "" {
		s = "asset"
	}
	if len(s) > 64 {
		s = s[:64]
		s = strings.TrimRight(s, "_")
	}
	if s == "" || !nameRe.MatchString(s) {
		return "asset"
	}
	return s
}

func (s *AssetService) allocateUniqueAssetName(base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "asset"
	}
	if !nameRe.MatchString(base) {
		return "", errors.New("素材名称只能包含字母、数字、下划线、短横线，长度 1-64")
	}
	for n := 1; n <= 99; n++ {
		cand := base
		if n > 1 {
			suf := fmt.Sprintf("_%d", n)
			room := 64 - len(suf)
			if room < 1 {
				return "", errors.New("无法生成可用资源名")
			}
			trunc := base
			if len(trunc) > room {
				trunc = trunc[:room]
			}
			trunc = strings.TrimRight(trunc, "_")
			if trunc == "" || !nameRe.MatchString(trunc+suf) {
				trunc = "asset"
				if len(trunc) > room {
					trunc = trunc[:room]
				}
				trunc = strings.TrimRight(trunc, "_")
				if trunc == "" {
					trunc = "a"
					if room < 1 {
						return "", errors.New("无法生成可用资源名")
					}
				}
			}
			cand = trunc + suf
		}
		if !nameRe.MatchString(cand) {
			continue
		}
		_, err := s.assetRepo.GetByName(cand)
		if errors.Is(err, sql.ErrNoRows) {
			return cand, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("无法生成可用资源名")
}