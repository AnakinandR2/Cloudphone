package phone

import (
	"io"
	"log"

	"golang.org/x/sync/errgroup"
	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
	"manager-backend/modules/library"
)

// pushFromLibraryDir 素材库推送的固定落地目录（与本地上传一致）。
const pushFromLibraryDir = "/sdcard/Download"

// pushFromLibraryMaxFiles 单次推送的文件数上限（与中台 batch-upload 限制一致）。
const pushFromLibraryMaxFiles = 10

// pushFromLibraryWorkers 下发阶段有界并发的 worker 数。
const pushFromLibraryWorkers = 5

// PushResult 单台云手机的推送结果（允许部分成功）。
type PushResult struct {
	PhoneID int    `json:"phone_id"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// PushFromLibrary 把素材库中的若干文件推送到一台或多台云手机的 /sdcard/Download。
//
// 流程（§4.2）：
//  1. 逐个手机属主校验（resolveCp）：任一不属主/未开通 → 整请求前置失败，不下发任何手机。
//  2. 下载一次：遍历 fileIDs 经 library 公开门面 OpenFileContent 直读私有 S3，io.ReadAll 后 Close，
//     组装 []midplat.UploadFile{Name, Data}。任一文件失败（锁定/不存在/非 active）→ 透传该错误（无副作用）。
//  3. 有界并发（=5）下发：每台经 s.ops.FileUpload(ctx, vmID, cpID, "/sdcard/Download", files)，
//     用 opCtxUpload() 超时；收集 per-phone {ok, error}。允许部分成功。
func (s *serviceImpl) PushFromLibrary(userID int, phoneIDs []int, fileIDs []uint) ([]PushResult, error) {
	if len(fileIDs) == 0 {
		return nil, apperr.BadRequest("未选择文件")
	}
	if len(fileIDs) > pushFromLibraryMaxFiles {
		return nil, apperr.BadRequest("一次最多推送 10 个文件")
	}
	if len(phoneIDs) == 0 {
		return nil, apperr.BadRequest("未选择云手机")
	}

	// 1) 逐个手机属主校验（任一不属主/未开通即整请求失败，无任何副作用）。
	cps := make([]*CloudPhone, 0, len(phoneIDs))
	for _, id := range phoneIDs {
		p, err := s.resolveCp(userID, id)
		if err != nil {
			return nil, err
		}
		cps = append(cps, p)
	}

	// 2) 下载一次：所有文件读进内存，多台复用（避免群控 N 台各拉一遍 S3）。
	files := make([]midplat.UploadFile, 0, len(fileIDs))
	for _, fid := range fileIDs {
		ref, err := library.OpenFileContent(userID, fid)
		if err != nil {
			log.Printf("[push-lib] OpenFileContent uid=%d fid=%d failed: %v", userID, fid, err)
			return nil, err
		}
		data, rerr := io.ReadAll(ref.Reader)
		_ = ref.Reader.Close()
		if rerr != nil {
			log.Printf("[push-lib] read content uid=%d fid=%d failed: %v", userID, fid, rerr)
			return nil, apperr.Internal("读取素材文件失败")
		}
		log.Printf("[push-lib] file fid=%d name=%q bytes=%d", fid, ref.Name, len(data))
		files = append(files, midplat.UploadFile{Name: ref.Name, Data: data})
	}

	// 3) 有界并发下发（worker<=5），逐台收集结果，允许部分成功。
	results := make([]PushResult, len(cps))
	eg := new(errgroup.Group)
	eg.SetLimit(pushFromLibraryWorkers)
	for i, p := range cps {
		i, p := i, p
		eg.Go(func() error {
			ctx, cancel := opCtxUpload()
			defer cancel()
			err := s.ops.FileUpload(ctx, p.VmID, p.CpID, pushFromLibraryDir, files)
			log.Printf("[push-lib] FileUpload phone=%d vmID=%s cpID=%s files=%d dir=%s err=%v",
				phoneIDs[i], p.VmID, p.CpID, len(files), pushFromLibraryDir, err)
			res := PushResult{PhoneID: phoneIDs[i], OK: err == nil}
			if err != nil {
				res.Error = err.Error()
			}
			results[i] = res
			return nil // 单台失败不影响其余台：部分成功
		})
	}
	_ = eg.Wait() // 各 goroutine 不返回错误（错误收进 results）
	return results, nil
}
