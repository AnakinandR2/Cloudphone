package phone

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
	"manager-backend/framework/s3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- 素材库 seeding（phone 测试不能 import library internal，直接写 DB）---

var libSeedSeq int

// seedLibraryFile 直接往 library_files 写一条 active 文件，返回其 id 与 s3_key。
func seedLibraryFile(t *testing.T, userID int, name string) (uint, string) {
	t.Helper()
	libSeedSeq++
	s3key := fmt.Sprintf("library/%d/test-%d", userID, libSeedSeq)
	err := framework.DB.Exec(
		`INSERT INTO library_files (user_id, folder_id, name, ext, mime_type, file_type, size_bytes, s3_key, status, created_at, updated_at)
		 VALUES (?, 0, ?, '', 'application/octet-stream', 'other', ?, ?, 'active', datetime('now'), datetime('now'))`,
		userID, name, len(name), s3key,
	).Error
	require.NoError(t, err)
	var id uint
	require.NoError(t, framework.DB.Raw(`SELECT id FROM library_files WHERE s3_key = ?`, s3key).Scan(&id).Error)
	require.NotZero(t, id)
	t.Cleanup(func() {
		framework.DB.Exec(`DELETE FROM library_files WHERE id = ?`, id)
	})
	return id, s3key
}

// lockLibraryUser 把某用户的素材库用量顶到超额（used>capacity）→ 触发取用锁定。
func lockLibraryUser(t *testing.T, userID int) {
	t.Helper()
	require.NoError(t, framework.DB.Exec(
		`INSERT INTO library_usage (user_id, used_bytes, updated_at) VALUES (?, ?, datetime('now'))`,
		userID, int64(1<<60),
	).Error)
	t.Cleanup(func() { framework.DB.Exec(`DELETE FROM library_usage WHERE user_id = ?`, userID) })
}

// fakeS3 起一个 path-style S3 兼容端点，GET 命中 objects[key] 返回字节。
func fakeS3(t *testing.T, objects map[string][]byte) *s3.Client {
	t.Helper()
	const bucket = "phone-lib-test"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/"+bucket+"/")
		body, ok := objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	cli, err := s3.New(s3.Config{
		Endpoint: srv.URL, Region: "us-east-1",
		AccessKeyID: "ak", SecretAccessKey: "sk",
		Bucket: bucket, UsePathStyle: true,
	})
	require.NoError(t, err)
	return cli
}

func withS3Library(t *testing.T, cli *s3.Client) {
	t.Helper()
	prev := framework.S3Library
	framework.S3Library = cli
	t.Cleanup(func() { framework.S3Library = prev })
}

// capturingOps 包住 fakePort，记录每次 FileUpload 收到的参数，并可让指定 cpID 失败。
type capturingOps struct {
	*fakePort
	mu        sync.Mutex
	uploads   []capturedUpload
	failCpIDs map[string]bool
}

type capturedUpload struct {
	vmID, cpID, folderPath string
	files                  []midplat.UploadFile
}

func (c *capturingOps) FileUpload(_ context.Context, vmID, cpID, folderPath string, files []midplat.UploadFile) error {
	c.mu.Lock()
	c.uploads = append(c.uploads, capturedUpload{vmID: vmID, cpID: cpID, folderPath: folderPath, files: files})
	c.mu.Unlock()
	if c.failCpIDs[cpID] {
		return errors.New("下发失败")
	}
	return nil
}

func newCapturingOps(fail ...string) *capturingOps {
	m := make(map[string]bool, len(fail))
	for _, f := range fail {
		m[f] = true
	}
	return &capturingOps{fakePort: &fakePort{}, failCpIDs: m}
}

// 多文件只下载一次 + 多台 fan-out 全部成功；断言每台收到相同的 []UploadFile。
func TestPushFromLibrary_MultiFileMultiPhone_AllOK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	ops := newCapturingOps()
	withFakeOps(t, ops)

	id1, k1 := seedLibraryFile(t, userA, "a.txt")
	id2, k2 := seedLibraryFile(t, userA, "b.txt")
	withS3Library(t, fakeS3(t, map[string][]byte{k1: []byte("AAA"), k2: []byte("BBBB")}))

	p1 := provisionedPhone(t, userA, "cp-1")
	p2 := provisionedPhone(t, userA, "cp-2")

	results, err := PhoneService.PushFromLibrary(userA, []int{p1, p2}, []uint{id1, id2})
	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.OK, "phone %d should be ok", r.PhoneID)
		assert.Empty(t, r.Error)
	}

	// 两台各收到一次下发，目录固定，文件内容一致（只下载一次、多台复用）。
	require.Len(t, ops.uploads, 2)
	for _, u := range ops.uploads {
		assert.Equal(t, "/sdcard/Download", u.folderPath)
		require.Len(t, u.files, 2)
		assert.Equal(t, "a.txt", u.files[0].Name)
		assert.Equal(t, []byte("AAA"), u.files[0].Data)
		assert.Equal(t, "b.txt", u.files[1].Name)
		assert.Equal(t, []byte("BBBB"), u.files[1].Data)
	}
	cps := []string{ops.uploads[0].cpID, ops.uploads[1].cpID}
	assert.ElementsMatch(t, []string{"cp-1", "cp-2"}, cps)
}

// 某台下发失败 → 部分成功结果（失败台标错，其余成功）。
func TestPushFromLibrary_OnePhoneFails_Partial(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	ops := newCapturingOps("cp-bad")
	withFakeOps(t, ops)

	id1, k1 := seedLibraryFile(t, userA, "f.bin")
	withS3Library(t, fakeS3(t, map[string][]byte{k1: []byte("data")}))

	pOK := provisionedPhone(t, userA, "cp-ok")
	pBad := provisionedPhone(t, userA, "cp-bad")

	results, err := PhoneService.PushFromLibrary(userA, []int{pOK, pBad}, []uint{id1})
	require.NoError(t, err)
	require.Len(t, results, 2)

	byID := map[int]PushResult{}
	for _, r := range results {
		byID[r.PhoneID] = r
	}
	assert.True(t, byID[pOK].OK)
	assert.False(t, byID[pBad].OK)
	assert.Equal(t, "下发失败", byID[pBad].Error)
}

// file_ids 超 10 / 为空 → 拒绝（不触达中台）。
func TestPushFromLibrary_FileIDsCountRejected(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	ops := newCapturingOps()
	withFakeOps(t, ops)
	withS3Library(t, fakeS3(t, map[string][]byte{}))
	p := provisionedPhone(t, userA, "cp-x")

	_, err := PhoneService.PushFromLibrary(userA, []int{p}, []uint{})
	require.Error(t, err)

	tooMany := make([]uint, 11)
	for i := range tooMany {
		tooMany[i] = uint(i + 1)
	}
	_, err = PhoneService.PushFromLibrary(userA, []int{p}, tooMany)
	require.Error(t, err)

	assert.Empty(t, ops.uploads, "拒绝时不应有任何下发")
}

// 含非属主 phone_id → 整请求拒绝，无任何下发。
func TestPushFromLibrary_NonOwnerPhoneRejected(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	ops := newCapturingOps()
	withFakeOps(t, ops)

	id1, k1 := seedLibraryFile(t, userA, "f.bin")
	withS3Library(t, fakeS3(t, map[string][]byte{k1: []byte("data")}))

	pA := provisionedPhone(t, userA, "cp-a")
	pB := provisionedPhone(t, userB, "cp-b") // 属于 B

	_, err := PhoneService.PushFromLibrary(userA, []int{pA, pB}, []uint{id1})
	require.Error(t, err) // A 不拥有 pB → 整请求失败
	assert.Empty(t, ops.uploads, "前置失败不应下发任何手机")
}

// 锁定用户（素材库超额）→ 整请求拒绝（下载阶段前置失败，无下发）。
func TestPushFromLibrary_LockedUserRejected(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones")
	})
	ops := newCapturingOps()
	withFakeOps(t, ops)

	id1, k1 := seedLibraryFile(t, userA, "f.bin")
	withS3Library(t, fakeS3(t, map[string][]byte{k1: []byte("data")}))
	lockLibraryUser(t, userA) // used>capacity → 取用锁定

	p := provisionedPhone(t, userA, "cp-a")

	_, err := PhoneService.PushFromLibrary(userA, []int{p}, []uint{id1})
	require.Error(t, err)
	assert.Empty(t, ops.uploads, "锁定时不应下发任何手机")
}
