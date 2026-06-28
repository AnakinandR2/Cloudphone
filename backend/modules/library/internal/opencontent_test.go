package library

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/apperr"
	"manager-backend/framework/s3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeS3Server 起一个最小 S3 兼容端点（path-style：/{bucket}/{key}）：
//   - GET 命中 objects[key] → 返回字节；未命中 → 404。
// 返回一个真实的 s3.Client（指向该端点），便于以 framework.S3Library 走完整 GetObject 链路。
func fakeS3Server(t *testing.T, objects map[string][]byte) *s3.Client {
	t.Helper()
	const bucket = "lib-test"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// path-style: /lib-test/<key...>
		key := strings.TrimPrefix(r.URL.Path, "/"+bucket+"/")
		body, ok := objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", itoaTest(len(body)))
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	cli, err := s3.New(s3.Config{
		Endpoint:        srv.URL,
		Region:          "us-east-1",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
		Bucket:          bucket,
		UsePathStyle:    true,
	})
	require.NoError(t, err)
	return cli
}

// withS3Library 临时把 framework.S3Library 换成给定 client，结束自动还原。
func withS3Library(t *testing.T, cli *s3.Client) {
	t.Helper()
	prev := framework.S3Library
	framework.S3Library = cli
	t.Cleanup(func() { framework.S3Library = prev })
}

// 正常：active 文件可读出字节流；字段透传正确。
func TestOpenFileContent_Success(t *testing.T) {
	const uid = 930101
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	now := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	setSub(t, uid, TierFree, 5*GiB, 0, nil)

	f := addFile(t, uid, FileActive, 11)
	f.Name = "hello.txt"
	f.MimeType = "text/plain"
	require.NoError(t, Service.repo.updateFile(f))

	withS3Library(t, fakeS3Server(t, map[string][]byte{f.S3Key: []byte("hello world")}))

	ref, err := Service.OpenFileContent(uid, f.ID)
	require.NoError(t, err)
	require.NotNil(t, ref.Reader)
	defer ref.Reader.Close()

	assert.Equal(t, f.ID, ref.FileID)
	assert.Equal(t, "hello.txt", ref.Name)
	assert.Equal(t, "text/plain", ref.MimeType)
	assert.Equal(t, int64(11), ref.SizeBytes)

	data, err := io.ReadAll(ref.Reader)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

// 不属主：别人 fileID 取不到（NotFound，不下载）。
func TestOpenFileContent_NotOwner(t *testing.T) {
	const owner = 930102
	const other = 930103
	cleanLibraryData(t, owner)
	cleanLibraryFiles(t, owner)
	cleanLibraryData(t, other)
	cleanLibraryFiles(t, other)
	fixedNow(t, time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC))
	setSub(t, owner, TierFree, 5*GiB, 0, nil)
	setSub(t, other, TierFree, 5*GiB, 0, nil)

	f := addFile(t, owner, FileActive, 5)
	withS3Library(t, fakeS3Server(t, map[string][]byte{f.S3Key: []byte("bytes")}))

	_, err := Service.OpenFileContent(other, f.ID)
	require.Error(t, err)
	assert.Equal(t, apperr.KindNotFound, apperr.KindOf(err))
}

// 超额锁定：used>capacity → 前置拒绝（取用语义）。
func TestOpenFileContent_Locked(t *testing.T) {
	const uid = 930104
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	fixedNow(t, time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC))
	setSub(t, uid, TierFree, 5*GiB, 0, nil)

	f := addFile(t, uid, FileActive, 8*GiB)
	require.NoError(t, Service.repo.setUsedBytes(uid, 8*GiB)) // 用量 8GiB > 容量 5GiB → 锁定
	withS3Library(t, fakeS3Server(t, map[string][]byte{f.S3Key: []byte("x")}))

	_, err := Service.OpenFileContent(uid, f.ID)
	require.Error(t, err)
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}

// 不存在：未知 fileID → NotFound。
func TestOpenFileContent_NotFound(t *testing.T) {
	const uid = 930105
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	fixedNow(t, time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC))
	setSub(t, uid, TierFree, 5*GiB, 0, nil)
	withS3Library(t, fakeS3Server(t, map[string][]byte{}))

	_, err := Service.OpenFileContent(uid, 99999999)
	require.Error(t, err)
	assert.Equal(t, apperr.KindNotFound, apperr.KindOf(err))
}

// 非 active（uploading/deleted）→ NotFound（不可取用）。
func TestOpenFileContent_NonActive(t *testing.T) {
	const uid = 930106
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	fixedNow(t, time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC))
	setSub(t, uid, TierFree, 5*GiB, 0, nil)

	f := addFile(t, uid, FileUploading, 5)
	withS3Library(t, fakeS3Server(t, map[string][]byte{f.S3Key: []byte("x")}))

	_, err := Service.OpenFileContent(uid, f.ID)
	require.Error(t, err)
	assert.Equal(t, apperr.KindNotFound, apperr.KindOf(err))
}

// S3 未配置 → 503/Unavailable。
func TestOpenFileContent_S3NotConfigured(t *testing.T) {
	const uid = 930107
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	fixedNow(t, time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC))
	setSub(t, uid, TierFree, 5*GiB, 0, nil)

	f := addFile(t, uid, FileActive, 5)
	withS3Library(t, nil) // 显式置空

	_, err := Service.OpenFileContent(uid, f.ID)
	require.Error(t, err)
	assert.Equal(t, apperr.KindUnavailable, apperr.KindOf(err))
}
