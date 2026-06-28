package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/s3"
	"manager-backend/modules/library"

	"github.com/stretchr/testify/require"
)

// libFileRow 是仅供测试用的最小 library_files 映射：app 模块 internal 不能 import
// library internal，故在测试包内本地声明同名表，直接造一条 active 的 app 类型文件，
// 让 library 公开门面（OpenFileContent / OpenFileWithTTL / ListUsableFiles）能读到它。
type libFileRow struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"column:user_id"`
	FolderID  uint   `gorm:"column:folder_id;default:0"`
	Name      string `gorm:"column:name"`
	Ext       string `gorm:"column:ext"`
	MimeType  string `gorm:"column:mime_type"`
	FileType  string `gorm:"column:file_type"`
	SizeBytes int64  `gorm:"column:size_bytes"`
	S3Key     string `gorm:"column:s3_key"`
	Status    string `gorm:"column:status"`
	MD5       string `gorm:"column:md5"`
}

func (libFileRow) TableName() string { return "library_files" }

// testUser 最小 users 表映射，供运营治理 join 上传者信息。
type testUser struct {
	ID       uint   `gorm:"primaryKey"`
	Phone    string `gorm:"column:phone"`
	Nickname string `gorm:"column:nickname"`
}

func (testUser) TableName() string { return "users" }

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	// 建本模块表 + users（运营治理 join）+ 装配 library（含其建表/定价 seed）。
	if err := framework.DB.AutoMigrate(&AppUserMeta{}, &AppMarket{}, &testUser{}); err != nil {
		panic(err)
	}
	if err := library.InitForTest(framework.DB); err != nil {
		panic(err)
	}
	AppService = newService(newRepository(framework.DB))
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}

// fakeStore 是 fakeS3 的内存对象表（并发安全）。
type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *fakeStore) put(k string, v []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[k] = v
}

func (s *fakeStore) get(k string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.objects[k]
	return v, ok
}

// fakeS3 起一个最小 S3 兼容端点（path-style：/{bucket}/{key}），支持 GET/PUT/HEAD，
// 返回一个真实 s3.Client。可选 publicBase 用于断言公有 URL 拼接。
func fakeS3(t *testing.T, bucket, publicBase string) (*s3.Client, *fakeStore) {
	t.Helper()
	store := &fakeStore{objects: map[string][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/"+bucket+"/")
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			store.put(key, b)
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			body, ok := store.get(key)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(http.StatusOK)
		default: // GET
			body, ok := store.get(key)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
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

// withS3 临时设置 framework.S3 / framework.S3Library，结束自动还原。
func withS3(t *testing.T, public, lib *s3.Client) {
	t.Helper()
	pp, pl := framework.S3, framework.S3Library
	framework.S3, framework.S3Library = public, lib
	t.Cleanup(func() { framework.S3, framework.S3Library = pp, pl })
}

// addLibFile 直接插入一条 active 的 app 类型素材库文件，S3Key 指向给定字节内容。
func addLibFile(t *testing.T, uid uint, name, key string, content []byte, store *fakeStore) uint {
	t.Helper()
	ext := ""
	if i := strings.LastIndex(name, "."); i >= 0 {
		ext = name[i+1:]
	}
	f := &libFileRow{
		UserID:    uid,
		Name:      name,
		Ext:       ext,
		MimeType:  "application/vnd.android.package-archive",
		FileType:  "app",
		SizeBytes: int64(len(content)),
		S3Key:     key,
		Status:    "active",
	}
	require.NoError(t, framework.DB.Create(f).Error)
	if store != nil {
		store.put(key, content)
	}
	return f.ID
}
