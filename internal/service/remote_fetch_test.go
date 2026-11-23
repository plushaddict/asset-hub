package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSuffixFromURLPath(t *testing.T) {
	if g := suffixFromURLPath("https://a.com/x/y.ICO?x=1"); g != "ico" {
		t.Fatalf("got %q", g)
	}
	if g := suffixFromURLPath("https://a.com/x/y"); g != "" {
		t.Fatalf("want empty, got %q", g)
	}
}

func TestSuffixFromContentType(t *testing.T) {
	cases := map[string]string{
		"image/png":                 "png",
		"image/jpeg; charset=utf-8": "jpg",
		"image/x-icon":              "ico",
		"image/vnd.microsoft.icon":  "ico",
		"text/html":                 "",
	}
	for in, want := range cases {
		if g := suffixFromContentType(in); g != want {
			t.Fatalf("%q: got %q want %q", in, g, want)
		}
	}
}

func TestFetchRemoteFile_OK_ico(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		_, _ = w.Write([]byte("ico-bytes"))
	}))
	defer srv.Close()
	remoteHTTPClient = srv.Client()
	defer func() { remoteHTTPClient = defaultRemoteHTTPClient() }()

	data, finalURL, ct, err := fetchRemoteFile(srv.URL + "/favicon/a.ico")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ico-bytes" || !strings.HasSuffix(finalURL, "/favicon/a.ico") {
		t.Fatalf("data=%q final=%q ct=%q", data, finalURL, ct)
	}
	suf, err := resolveRemoteSuffix(finalURL, ct)
	if err != nil || suf != "ico" {
		t.Fatalf("suf=%q err=%v", suf, err)
	}
}

func TestFetchRemoteFile_RedirectUsesFinalPath(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	final := srv.URL + "/real/icon.png"
	mux.HandleFunc("/go", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final, http.StatusFound)
	})
	mux.HandleFunc("/real/icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("png"))
	})
	remoteHTTPClient = srv.Client()
	defer func() { remoteHTTPClient = defaultRemoteHTTPClient() }()

	_, finalURL, ct, err := fetchRemoteFile(srv.URL + "/go")
	if err != nil {
		t.Fatal(err)
	}
	suf, err := resolveRemoteSuffix(finalURL, ct)
	if err != nil || suf != "png" {
		t.Fatalf("want png from final path, got %q ct=%q err=%v final=%q", suf, ct, err, finalURL)
	}
}

func TestFetchRemoteFile_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = io.WriteString(w, "nope")
	}))
	defer srv.Close()
	remoteHTTPClient = srv.Client()
	defer func() { remoteHTTPClient = defaultRemoteHTTPClient() }()

	_, _, _, err := fetchRemoteFile(srv.URL + "/missing")
	if err == nil || !strings.Contains(err.Error(), "远程文件下载失败") || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err=%v", err)
	}
}

func TestFetchRemoteFile_BadScheme(t *testing.T) {
	_, _, _, err := fetchRemoteFile("ftp://x/a.png")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchRemoteFile_TooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, MaxUploadSize+1))
	}))
	defer srv.Close()
	remoteHTTPClient = srv.Client()
	defer func() { remoteHTTPClient = defaultRemoteHTTPClient() }()

	_, _, _, err := fetchRemoteFile(srv.URL + "/big.png")
	if err == nil || !strings.Contains(err.Error(), "20MB") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveRemoteSuffix_ContentTypeFallback(t *testing.T) {
	suf, err := resolveRemoteSuffix("https://cdn.example/x", "image/png")
	if err != nil || suf != "png" {
		t.Fatalf("%q %v", suf, err)
	}
}