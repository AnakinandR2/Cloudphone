package library

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/apperr"

	"github.com/google/uuid"
)

// presignGetTTL / presignPutTTL 取服务注入的可配 TTL（来自 framework.AppConfig，
// 在 module.Init 注入）；注入值缺失（<=0，如未经 LoadConfig 的极端测试路径）时回退默认 15m/30m。
func (s *serviceImpl) presignGetTTL() time.Duration {
	if s.getTTL > 0 {
		return s.getTTL
	}
	return 15 * time.Minute
}

func (s *serviceImpl) presignPutTTL() time.Duration {
	if s.putTTL > 0 {
		return s.putTTL
	}
	return 30 * time.Minute
}

// errS3NotConfigured S3 未配置时的统一错误（HTTP 503）。
func errS3NotConfigured() error {
	return apperr.New(apperr.KindUnavailable, "对象存储未配置，暂无法上传或下载文件")
}

// ---- 配额与锁定（§6）----

// assertNotLocked 超额锁定校验：used_bytes > capacity_bytes 时拒绝取用（下载 / OpenFile 都调）。
func (s *serviceImpl) assertNotLocked(userID int) error {
	cur, err := s.currentSubscription(userID)
	if err != nil {
		return err
	}
	usage, err := s.repo.getUsage(userID)
	if err != nil {
		return err
	}
	if usage.UsedBytes > cur.capacity {
		return apperr.New(apperr.KindForbidden, "容量超限，请扩容或删除文件后再使用")
	}
	return nil
}

// reconcileUsage 由 sum(active files.size_bytes) 重算并覆盖 used_bytes。
func (s *serviceImpl) reconcileUsage(userID int) (int64, error) {
	sum, err := s.repo.sumActiveSize(userID)
	if err != nil {
		return 0, err
	}
	if err := s.repo.setUsedBytes(userID, sum); err != nil {
		return 0, err
	}
	return sum, nil
}

// ---- 上传（§5）----

// PresignUploadRequest 上传第一段请求体。
// MD5（全量内容 md5）+ SliceMD5（md5(前 256KB)）用于全局去重秒传查重（§4、§5）。
type PresignUploadRequest struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	Mime      string `json:"mime"`
	FolderID  uint   `json:"folder_id"`
	MD5       string `json:"md5"`
	SliceMD5  string `json:"slice_md5"`
}

// PresignUploadResult 返回 PUT 直传地址 + file_id；秒传命中时 Instant=true 且无 upload_url。
type PresignUploadResult struct {
	FileID    uint   `json:"file_id"`
	UploadURL string `json:"upload_url,omitempty"`
	S3Key     string `json:"s3_key,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"` // 秒
	Instant   bool   `json:"instant"`              // true=秒传命中，文件已 active，前端跳过 PUT/confirm
}

// dedupFileName 同夹下重名时在扩展名前附加 _<id>，如 video.mp4 → video_123.mp4、noext → noext_5。
func dedupFileName(name string, id uint) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return base + "_" + strconv.Itoa(int(id)) + ext
}

// PresignUpload 上传第一段：配额校验（active+uploading 声明 + 本次 ≤ capacity）→ 建 uploading 行 → 签发 PUT URL。
func (s *serviceImpl) PresignUpload(userID int, req PresignUploadRequest) (*PresignUploadResult, error) {
	if framework.S3Library == nil {
		return nil, errS3NotConfigured()
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperr.Validation("文件名不能为空")
	}
	if req.SizeBytes <= 0 {
		return nil, apperr.Validation("文件大小非法")
	}
	// 文件夹归属校验（0=根免校验）。
	if req.FolderID != 0 {
		fo, err := s.repo.getFolder(userID, req.FolderID)
		if err != nil {
			return nil, err
		}
		if fo == nil {
			return nil, apperr.NotFound("文件夹不存在")
		}
	}
	// 配额上限（容量）。
	cur, err := s.currentSubscription(userID)
	if err != nil {
		return nil, err
	}

	md5 := strings.ToLower(strings.TrimSpace(req.MD5))
	sliceMD5 := strings.ToLower(strings.TrimSpace(req.SliceMD5))

	ext := strings.ToLower(filepath.Ext(req.Name))
	s3key := "library/" + strconv.Itoa(userID) + "/" + uuid.NewString() + ext
	f := &LibraryFile{
		UserID:    uint(userID),
		FolderID:  req.FolderID,
		Name:      req.Name,
		Ext:       strings.TrimPrefix(ext, "."),
		MimeType:  req.Mime,
		FileType:  classifyFileType(ext, req.Mime),
		SizeBytes: req.SizeBytes, // 声明值，confirm 时以真实大小修正
		S3Key:     s3key,
		Status:    FileUploading,
		MD5:       md5,
		SliceMD5:  sliceMD5,
	}
	// major-3：配额校验（active+uploading 声明 + 本次 ≤ capacity）与建 uploading 行包进单事务，
	// 事务内重算 SUM 后再 INSERT，关闭 TOCTOU 超卖窗口。md5/slice_md5 随行落库。
	if err := s.repo.reserveUpload(userID, f, cur.capacity); err != nil {
		return nil, err
	}
	// 同夹下重名 → 自动加 _<自增id> 保证唯一。f.ID 在 reserveUpload(INSERT) 后已赋值。
	if dup, _ := s.repo.fileNameExists(userID, req.FolderID, f.Name, f.ID); dup {
		f.Name = dedupFileName(f.Name, f.ID)
		f.Ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Name)), ".")
		if err := s.repo.updateFile(f); err != nil {
			return nil, err
		}
	}

	// 秒传查重（§5.2）：blob (md5,size) 存在且 slice_md5 一致 → 跳过上传，瞬时建立引用。
	// slice_md5 不一致（仅声明 md5）→ 不秒传，回退真实上传（§2 防护）。
	if md5 != "" {
		if blob, err := s.repo.getBlobByMD5Size(md5, req.SizeBytes); err == nil &&
			blob != nil && blob.SliceMD5 != "" && blob.SliceMD5 == sliceMD5 {
			if res, ok := s.tryInstantHit(userID, f, blob, cur.capacity); ok {
				return res, nil
			}
			// 秒传 finalize 失败（如越容量、blob 已被并发删）→ 回退真实上传。
		}
	}

	putTTL := s.presignPutTTL()
	url, err := framework.S3Library.PresignPutURL(context.Background(), s3key, putTTL)
	if err != nil {
		// 签发失败 → 回收占位行，避免泄漏预占。
		_ = s.repo.hardDeleteFile(f.ID)
		return nil, apperr.Internal("生成上传地址失败")
	}
	return &PresignUploadResult{
		FileID:    f.ID,
		UploadURL: url,
		S3Key:     s3key,
		ExpiresIn: int(putTTL.Seconds()),
		Instant:   false,
	}, nil
}

// tryInstantHit 秒传命中收尾：把 uploading 文件行收编到既有 blob —— 指向 blob 的物理 key、
// blob.ref_count++、文件直接 active 并按 blob 真实大小 used_bytes+=size（与 confirm 同款记账）。
// 成功返回 (result, true)；任一步失败回退（ref 已加则回退减一），返回 (nil, false) 让调用方走真实上传。
func (s *serviceImpl) tryInstantHit(userID int, f *LibraryFile, blob *LibraryBlob, capacity int64) (*PresignUploadResult, bool) {
	// 保存原始（reserveUpload 建行时的）temp key 与声明大小，失败时据此把行回滚干净。
	origKey, origSize := f.S3Key, f.SizeBytes
	if err := s.repo.incRef(blob.ID); err != nil {
		return nil, false
	}
	f.BlobID = blob.ID
	f.S3Key = blob.S3Key
	f.SizeBytes = blob.SizeBytes
	if err := s.repo.updateFile(f); err != nil {
		// updateFile 失败：DB 未持久化改动；回滚内存字段后回退真实上传。
		_ = s.repo.decRef(blob.ID)
		f.BlobID, f.S3Key, f.SizeBytes = 0, origKey, origSize
		return nil, false
	}
	// 与 confirm 同款记账：以 blob 真实大小置 active + 校验容量 + used_bytes+=size。
	if err := s.repo.confirmActiveWithinCapacity(userID, f.ID, blob.SizeBytes, capacity); err != nil {
		// 秒传 finalize 失败（如越容量）：先减引用，再把已持久化的行回滚为干净的 uploading 行
		// （BlobID=0、S3Key=原 temp、size=原声明）。否则残留脏行会造成两类共享对象损坏：
		//  ① 调用方回退真实上传时拿到被改写成 blob 共享 key 的行 → confirm 误删共享对象；
		//  ② cron 的 sweepStaleUploads 对 BlobID>0 残留行再 decRef 一次 → 与此处重复、
		//     ref_count 虚低 → 孤儿清理误删仍被他人引用的共享对象。
		_ = s.repo.decRef(blob.ID)
		f.BlobID, f.S3Key, f.SizeBytes, f.Status = 0, origKey, origSize, FileUploading
		_ = s.repo.updateFile(f)
		return nil, false
	}
	return &PresignUploadResult{FileID: f.ID, Instant: true}, true
}

// ConfirmUploadRequest 上传第二段请求体。
type ConfirmUploadRequest struct {
	FileID uint   `json:"file_id"`
	TagIDs []uint `json:"tag_ids"`
}

// ConfirmUpload 上传第二段（§5.4）：Head(tempKey) 取真实大小 + ETag 权威校验 md5
// → blob 行锁内合并（指向既有 blob、删 tempKey、ref++）或收编（以 tempKey 建 blob、ref=1）
// → 修正大小 + active + used_bytes+=size → 绑标签。
func (s *serviceImpl) ConfirmUpload(userID int, req ConfirmUploadRequest) (*LibraryFile, error) {
	if framework.S3Library == nil {
		return nil, errS3NotConfigured()
	}
	f, err := s.repo.getFile(userID, req.FileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, apperr.NotFound("文件不存在")
	}
	if f.Status == FileActive {
		return f, nil // 幂等：已确认过直接返回（秒传/已 confirm）
	}
	if f.Status != FileUploading {
		return nil, apperr.Validation("文件状态不允许确认")
	}
	tempKey := f.S3Key
	// Head(tempKey) → (realSize, etag)。对象不存在/取不到 → 删行 + 提示重传。
	head, err := framework.S3Library.Head(context.Background(), tempKey)
	if err != nil {
		_ = s.repo.hardDeleteFile(f.ID)
		return nil, apperr.Validation("未检测到上传文件，请重新上传")
	}
	realSize := head.Size
	// §6 权威校验：真实大小须等于声明大小；md5 取 ETag（单段 PUT 即内容 md5），
	// ETag 非 32-hex（分段上传）→ 回退 GetObject 流式算 md5。不符即删对象 + 拒绝。
	if f.MD5 != "" {
		realMD5 := strings.ToLower(head.ETag)
		if !isHex32(realMD5) {
			realMD5, err = s.streamMD5(tempKey)
			if err != nil {
				_ = s.deleteTempAndRow(f.ID, tempKey)
				return nil, apperr.Validation("校验上传文件失败，请重新上传")
			}
		}
		if realSize != f.SizeBytes || realMD5 != f.MD5 {
			_ = s.deleteTempAndRow(f.ID, tempKey)
			return nil, apperr.Validation("上传文件校验不通过（大小或内容不一致），请重新上传")
		}
	} else if realSize != f.SizeBytes {
		// 无声明 md5（兼容/老前端）：至少校验大小。
		_ = s.deleteTempAndRow(f.ID, tempKey)
		return nil, apperr.Validation("上传文件大小不一致，请重新上传")
	}

	// blob 行锁内决定合并 / 收编（§5.4、§8）。完成后 f.BlobID/f.S3Key 已就位。
	if err := s.resolveBlobForConfirm(f, tempKey, realSize); err != nil {
		_ = s.deleteTempAndRow(f.ID, tempKey)
		return nil, err
	}

	// major-3：以真实大小修正 + 置 active + 校验容量 + used_bytes+=size（记账与现状一致）。
	cur, err := s.currentSubscription(userID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.confirmActiveWithinCapacity(userID, f.ID, realSize, cur.capacity); err != nil {
		// 越容量：减引用 + 删行；blob 对象由孤儿 blob 清理（ref→0）负责，不在此直接删。
		if f.BlobID > 0 {
			_ = s.repo.decRef(f.BlobID)
		}
		_ = s.repo.hardDeleteFile(f.ID)
		return nil, err
	}
	f.SizeBytes = realSize
	f.Status = FileActive
	if len(req.TagIDs) > 0 {
		valid := s.filterOwnedTags(userID, req.TagIDs)
		if err := s.repo.setFileTags(f.ID, valid); err != nil {
			return nil, err
		}
	}
	return f, nil
}

// resolveBlobForConfirm 在 blob (md5,size) 维度收编/合并物理对象（§5.4）：
//   - blob 已存在 → 合并：删 tempKey、文件指向 blob.S3Key、ref++。
//   - 不存在 → 收编：以 tempKey 建 blob{ref=1}，文件 BlobID=blob.id（S3Key 不变，已等于 tempKey）。
//   - 并发首传唯一键冲突 → 回退合并分支（回读 blob + ref++ + 删 tempKey）。
//
// 无声明 md5（BlobID 保持 0）走旧的「文件自有 S3Key」路径，不建 blob。
func (s *serviceImpl) resolveBlobForConfirm(f *LibraryFile, tempKey string, realSize int64) error {
	if f.MD5 == "" {
		return nil // 存量/兼容：不去重
	}
	blob, err := s.repo.getBlobByMD5Size(f.MD5, realSize)
	if err != nil {
		return err
	}
	if blob != nil {
		return s.mergeIntoBlob(f, blob, tempKey)
	}
	// 收编：以 tempKey 建 blob。
	nb := &LibraryBlob{
		MD5:       f.MD5,
		SizeBytes: realSize,
		SliceMD5:  f.SliceMD5,
		S3Key:     tempKey,
		RefCount:  1,
	}
	if err := s.repo.createBlob(nb); err != nil {
		if errors.Is(err, errBlobConflict) {
			// 并发首传：另一方已建成 → 回读 + 合并（删 tempKey + ref++）。
			existing, gerr := s.repo.getBlobByMD5Size(f.MD5, realSize)
			if gerr != nil {
				return gerr
			}
			if existing == nil {
				return apperr.Internal("内容去重冲突处理失败")
			}
			return s.mergeIntoBlob(f, existing, tempKey)
		}
		return err
	}
	f.BlobID = nb.ID
	// S3Key 不变（已等于 tempKey）；持久化 BlobID。
	if err := s.repo.updateFile(f); err != nil {
		// 收编已建 blob（ref=1）但绑定到文件失败：此刻尚无任何文件指向该 blob，
		// ref=1 仅来自刚建的 blob 自身，无并发引用风险。回滚硬删该 blob 行，
		// 否则它将以 ref_count=1 永久滞留（孤儿 blob 清理只扫 ref<=0，永不回收），
		// 而外层 ConfirmUpload 的 deleteTempAndRow 会删掉 tempKey 物理对象 →
		// 后续相同内容上传命中该 blob 时指向已失踪的对象。失败仅记日志、不覆盖原始 err。
		f.BlobID = 0
		if derr := s.repo.deleteBlob(nb.ID); derr != nil {
			log.Printf("[library] resolveBlobForConfirm: rollback orphan blob %d failed: %v", nb.ID, derr)
		}
		return err
	}
	return nil
}

// mergeIntoBlob 合并分支：删自身 tempKey、文件指向既有 blob 的物理 key、blob.ref_count++。
func (s *serviceImpl) mergeIntoBlob(f *LibraryFile, blob *LibraryBlob, tempKey string) error {
	if err := s.repo.incRef(blob.ID); err != nil {
		return err
	}
	if framework.S3Library != nil && tempKey != "" && tempKey != blob.S3Key {
		_ = framework.S3Library.DeleteObject(context.Background(), tempKey)
	}
	f.BlobID = blob.ID
	f.S3Key = blob.S3Key
	if err := s.repo.updateFile(f); err != nil {
		_ = s.repo.decRef(blob.ID)
		return err
	}
	return nil
}

// deleteTempAndRow 删临时对象 + 删文件行（confirm 校验失败/越容量的回收）。
func (s *serviceImpl) deleteTempAndRow(fileID uint, tempKey string) error {
	if framework.S3Library != nil && tempKey != "" {
		_ = framework.S3Library.DeleteObject(context.Background(), tempKey)
	}
	return s.repo.hardDeleteFile(fileID)
}

// streamMD5 GetObject 流式算对象内容 md5（ETag 非 32-hex 的分段上传回退路径，§6）。
func (s *serviceImpl) streamMD5(key string) (string, error) {
	rc, err := framework.S3Library.GetObject(context.Background(), key)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	h := md5.New()
	if _, err := io.Copy(h, rc); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// isHex32 判断字符串是否为 32 位十六进制（合法 md5）。
func isHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// ---- 列表 / 详情 ----

// FileDTO 文件返回（含标签 ID）。
type FileDTO struct {
	LibraryFile
	TagIDs []uint `json:"tag_ids"`
}

// ListFiles 文件列表（§8）。
func (s *serviceImpl) ListFiles(userID int, f fileListFilter) ([]FileDTO, int64, error) {
	list, total, err := s.repo.listFiles(userID, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]FileDTO, 0, len(list))
	for i := range list {
		ids, err := s.repo.listFileTagIDs(list[i].ID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, FileDTO{LibraryFile: list[i], TagIDs: ids})
	}
	return out, total, nil
}

// DownloadFile 取下载 presigned GET（先做锁定校验，§6）。
func (s *serviceImpl) DownloadFile(userID int, fileID uint) (string, error) {
	if framework.S3Library == nil {
		return "", errS3NotConfigured()
	}
	if err := s.assertNotLocked(userID); err != nil {
		return "", err
	}
	f, err := s.repo.getFile(userID, fileID)
	if err != nil {
		return "", err
	}
	if f == nil || f.Status != FileActive {
		return "", apperr.NotFound("文件不存在")
	}
	return framework.S3Library.PresignGetURL(context.Background(), f.S3Key, s.presignGetTTL())
}

// ---- 改名 / 移动 / 删除 ----

// UpdateFileRequest 改名/移动请求体（字段为指针，nil 表示不改）。
type UpdateFileRequest struct {
	Name     *string `json:"name"`
	FolderID *uint   `json:"folder_id"`
}

// UpdateFile 重命名 / 移动文件夹。
func (s *serviceImpl) UpdateFile(userID int, fileID uint, req UpdateFileRequest) (*LibraryFile, error) {
	f, err := s.repo.getFile(userID, fileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, apperr.NotFound("文件不存在")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperr.Validation("文件名不能为空")
		}
		f.Name = name
	}
	if req.FolderID != nil {
		if *req.FolderID != 0 {
			fo, err := s.repo.getFolder(userID, *req.FolderID)
			if err != nil {
				return nil, err
			}
			if fo == nil {
				return nil, apperr.NotFound("目标文件夹不存在")
			}
		}
		f.FolderID = *req.FolderID
	}
	if err := s.repo.updateFile(f); err != nil {
		return nil, err
	}
	return f, nil
}

// DeleteFile 软删（cron 异步删 S3）；同时回收 used_bytes（仅 active 计入过用量）。
func (s *serviceImpl) DeleteFile(userID int, fileID uint) error {
	f, err := s.repo.getFile(userID, fileID)
	if err != nil {
		return err
	}
	if f == nil {
		return apperr.NotFound("文件不存在")
	}
	now := s.now()
	if err := s.repo.softDeleteFile(userID, fileID, now); err != nil {
		return err
	}
	if f.Status == FileActive {
		if err := s.repo.addUsedBytes(userID, -f.SizeBytes); err != nil {
			return err
		}
	}
	// 去重文件（blob_id>0）：减引用（行锁内）。归零后由 cron 孤儿清理真删 S3（§7）。
	if f.BlobID > 0 {
		if err := s.repo.decRef(f.BlobID); err != nil {
			return err
		}
	}
	return nil
}

// ---- 对外"取文件"门面（§7）----

// FileRef 一个可用文件的引用（含短时有效的 presigned GET URL）。
type FileRef struct {
	FileID    uint
	Name      string
	MimeType  string
	SizeBytes int64
	URL       string
}

// UsableFilter 消费侧选择器过滤（按类型/标签）。
type UsableFilter struct {
	FileType string // image|video|audio|document|other；空=不限
	TagID    uint   // >0 时按标签过滤
	Keyword  string
	Page     int
	Size     int
}

// OpenFile 取一个可用文件：属主校验 + 超额锁定校验 → presigned GET（默认 GET TTL）。
// 委托给 OpenFileWithTTL，传服务注入的默认 GET TTL（presignGetTTL）。
func (s *serviceImpl) OpenFile(userID int, fileID uint) (FileRef, error) {
	return s.OpenFileWithTTL(userID, fileID, s.presignGetTTL())
}

// OpenFileWithTTL 取一个可用文件：与 OpenFile 校验完全一致（属主校验 + 超额锁定校验 +
// 仅 active），只是 presign GET 用传入的 ttl 而非默认 GET TTL。
// 应用按 URL 安装场景须够中台下载完成，故传更长的 install TTL（见 §7.4）。
func (s *serviceImpl) OpenFileWithTTL(userID int, fileID uint, ttl time.Duration) (FileRef, error) {
	if framework.S3Library == nil {
		return FileRef{}, errS3NotConfigured()
	}
	if err := s.assertNotLocked(userID); err != nil {
		return FileRef{}, err
	}
	f, err := s.repo.getFile(userID, fileID)
	if err != nil {
		return FileRef{}, err
	}
	if f == nil || f.Status != FileActive {
		return FileRef{}, apperr.NotFound("文件不存在")
	}
	url, err := framework.S3Library.PresignGetURL(context.Background(), f.S3Key, ttl)
	if err != nil {
		return FileRef{}, apperr.Internal("生成访问地址失败")
	}
	return FileRef{
		FileID:    f.ID,
		Name:      f.Name,
		MimeType:  f.MimeType,
		SizeBytes: f.SizeBytes,
		URL:       url,
	}, nil
}

// FileContentRef 一个可用文件的内容流（调用方负责 Close Reader）。
type FileContentRef struct {
	FileID    uint
	Name      string
	MimeType  string
	SizeBytes int64
	Reader    io.ReadCloser
}

// OpenFileContent 取一个可用文件的字节流：属主校验 + 超额锁定校验 + 仅 active → S3 GetObject。
// 与 OpenFile 校验完全一致，只把「presign 出 URL」换成「GetObject 出流」，避免后端再对 presigned URL
// 发一次 HTTP（私有桶字节不经浏览器，直读私有 S3 → 中台下发）。调用方负责 Close 返回的 Reader。
func (s *serviceImpl) OpenFileContent(userID int, fileID uint) (FileContentRef, error) {
	if framework.S3Library == nil {
		return FileContentRef{}, errS3NotConfigured()
	}
	if err := s.assertNotLocked(userID); err != nil {
		return FileContentRef{}, err
	}
	f, err := s.repo.getFile(userID, fileID)
	if err != nil {
		return FileContentRef{}, err
	}
	if f == nil || f.Status != FileActive {
		return FileContentRef{}, apperr.NotFound("文件不存在")
	}
	rc, err := framework.S3Library.GetObject(context.Background(), f.S3Key)
	if err != nil {
		return FileContentRef{}, apperr.Internal("读取文件内容失败")
	}
	return FileContentRef{
		FileID:    f.ID,
		Name:      f.Name,
		MimeType:  f.MimeType,
		SizeBytes: f.SizeBytes,
		Reader:    rc,
	}, nil
}

// ListUsableFiles 列出可用文件（消费侧选择器用）。锁定时拒绝（取用语义）。
func (s *serviceImpl) ListUsableFiles(userID int, filter UsableFilter) ([]FileRef, error) {
	if framework.S3Library == nil {
		return nil, errS3NotConfigured()
	}
	if err := s.assertNotLocked(userID); err != nil {
		return nil, err
	}
	list, _, err := s.repo.listFiles(userID, fileListFilter{
		FileType: filter.FileType,
		TagID:    filter.TagID,
		Keyword:  filter.Keyword,
		Page:     filter.Page,
		Size:     filter.Size,
	})
	if err != nil {
		return nil, err
	}
	getTTL := s.presignGetTTL()
	out := make([]FileRef, 0, len(list))
	for i := range list {
		url, err := framework.S3Library.PresignGetURL(context.Background(), list[i].S3Key, getTTL)
		if err != nil {
			return nil, apperr.Internal("生成访问地址失败")
		}
		out = append(out, FileRef{
			FileID:    list[i].ID,
			Name:      list[i].Name,
			MimeType:  list[i].MimeType,
			SizeBytes: list[i].SizeBytes,
			URL:       url,
		})
	}
	return out, nil
}

// ---- helpers ----

// filterOwnedTags 过滤出属于该用户的标签 ID。
func (s *serviceImpl) filterOwnedTags(userID int, ids []uint) []uint {
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		t, err := s.repo.getTag(userID, id)
		if err == nil && t != nil {
			out = append(out, id)
		}
	}
	return out
}

// classifyFileType 由扩展名 / mime 归类（image|video|audio|document|app|other）。
func classifyFileType(ext, mime string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	mime = strings.ToLower(mime)
	switch {
	case strings.HasPrefix(mime, "image/"):
		return FileTypeImage
	case strings.HasPrefix(mime, "video/"):
		return FileTypeVideo
	case strings.HasPrefix(mime, "audio/"):
		return FileTypeAudio
	case strings.Contains(mime, "android.package-archive"):
		return FileTypeApp
	}
	switch ext {
	case "apk", "xapk":
		return FileTypeApp
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "heic":
		return FileTypeImage
	case "mp4", "mov", "avi", "mkv", "webm", "flv", "wmv", "m4v":
		return FileTypeVideo
	case "mp3", "wav", "flac", "aac", "ogg", "m4a":
		return FileTypeAudio
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "csv", "md":
		return FileTypeDocument
	}
	return FileTypeOther
}
