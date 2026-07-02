package apptest

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/s3"

	"github.com/stretchr/testify/require"
)

// s3fake_test.go 移植自 modules/app/internal/main_test.go 的 fakeS3/fakeStore/withS3，
// 改为 apptest 包内共享辅助（前缀 fake/newFakeS3/withFakeS3 避免与他处冲突）。
// 起一个最小 S3 兼容端点（path-style：/{bucket}/{key}）供集成测试注入到
// framework.S3 / framework.S3Library，验证真实上传/确认/下载链路（走公开 HTTP + presigned URL）。

// fakeS3Store 是 newFakeS3 的内存对象表（并发安全）。
type fakeS3Store struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *fakeS3Store) put(k string, v []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[k] = v
}

func (s *fakeS3Store) get(k string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.objects[k]
	return v, ok
}

func (s *fakeS3Store) del(k string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, k)
}

// etagOf 返回单段 PUT 语义下的 ETag：对象内容 md5 的 32-hex，两端加引号（与真实 S3 一致）。
// framework/s3.Head 会 strings.Trim 掉两端引号后回给 library.ConfirmUpload 做 md5 权威校验，
// 故此处必须带引号，才能真实走「ETag==md5」主校验路径（而非 GetObject 流式回退）。
func etagOf(b []byte) string {
	sum := md5.Sum(b)
	return "\"" + hex.EncodeToString(sum[:]) + "\""
}

// newFakeS3 起一个最小 S3 兼容端点（path-style：/{bucket}/{key}），支持 PUT/HEAD/GET，
// 返回一个真实 s3.Client。HEAD/PUT 均回带 ETag（= 存储字节 md5，带引号），使
// library.ConfirmUpload 走「Head.ETag == md5」主校验路径。可选 publicBase 供公有 URL 拼接。
func newFakeS3(t *testing.T, bucket, publicBase string) (*s3.Client, *fakeS3Store) {
	t.Helper()
	store := &fakeS3Store{objects: map[string][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/"+bucket+"/")
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			store.put(key, b)
			w.Header().Set("ETag", etagOf(b))
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			body, ok := store.get(key)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Header().Set("ETag", etagOf(body))
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			// 真删对象（不再是空操作）：让「confirm 失败删临时对象」「市场批量删除公有桶对象」
			// 这类断言能真实验证 DeleteObject 已生效。
			store.del(key)
			w.WriteHeader(http.StatusNoContent)
		default: // GET
			body, ok := store.get(key)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", etagOf(body))
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	cli, err := s3.New(s3.Config{
		Endpoint:        srv.URL,
		Region:          "us-east-1",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
		Bucket:          bucket,
		UsePathStyle:    true,
		PublicBaseURL:   publicBase,
	})
	require.NoError(t, err)
	return cli, store
}

// withFakeS3 临时设置 framework.S3（公有桶）/ framework.S3Library（私有桶），结束自动还原。
// 传 nil 表示保持该项为「未配置」。
func withFakeS3(t *testing.T, public, lib *s3.Client) {
	t.Helper()
	pp, pl := framework.S3, framework.S3Library
	framework.S3, framework.S3Library = public, lib
	t.Cleanup(func() { framework.S3, framework.S3Library = pp, pl })
}
