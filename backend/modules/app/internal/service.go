package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/apperr"
	"manager-backend/modules/app/internal/apkparse"
	"manager-backend/modules/library"

	"github.com/google/uuid"
)

// AppService 模块内服务实例，由 module.Init 注入后装配。
var AppService *serviceImpl

type serviceImpl struct {
	repo repository
}

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// installTTL 从配置取应用按 URL 安装专用的 presigned GET 有效期（见 §7.4）；缺配置回退 1h。
func installTTL() time.Duration {
	if framework.AppConfig != nil && framework.AppConfig.S3LibraryInstallGetTTL > 0 {
		return framework.AppConfig.S3LibraryInstallGetTTL
	}
	return time.Hour
}

// ---- DTO ----

// UserAppDTO 是「我的应用」列表/详情条目（library app 文件 + app_user_meta）。
type UserAppDTO struct {
	FileID      uint   `json:"file_id"`
	AppName     string `json:"app_name"`
	PackageName string `json:"package_name"`
	Version     string `json:"version"`
	IconURL     string `json:"icon_url"`
	SizeBytes   int64  `json:"size_bytes"`
	ParseStatus string `json:"parse_status"`
	ParseError  string `json:"parse_error"`
	CreatedAt   string `json:"created_at"`
}

// MarketAppDTO 是应用市场条目（用户端浏览 / admin 管理共用）。
type MarketAppDTO struct {
	ID          uint   `json:"id"`
	AppName     string `json:"app_name"`
	PackageName string `json:"package_name"`
	Version     string `json:"version"`
	IconURL     string `json:"icon_url"`
	SizeBytes   int64  `json:"size_bytes"`
	ParseStatus string `json:"parse_status"`
	ParseError  string `json:"parse_error"`
	CreatedAt   string `json:"created_at"`
}

// AdminUserAppDTO 是运营治理条目：用户应用文件 + 上传者 + 解析元数据。
type AdminUserAppDTO struct {
	UserAppDTO
	UserID       uint   `json:"user_id"`
	UserPhone    string `json:"user_phone"`
	UserNickname string `json:"user_nickname"`
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ---- 用户应用 ----

// ListUserApps 列出当前用户的 app 类型素材库文件，LEFT JOIN app_user_meta 组 DTO。
func (s *serviceImpl) ListUserApps(userID, page, size int) ([]UserAppDTO, error) {
	files, err := library.ListUsableFiles(userID, library.UsableFilter{
		FileType: "app", Page: page, Size: size,
	})
	if err != nil {
		return nil, err
	}
	fileIDs := make([]uint, 0, len(files))
	for i := range files {
		fileIDs = append(fileIDs, files[i].FileID)
	}
	metas, err := s.repo.getUserMetaByIDs(fileIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]AppUserMeta, len(metas))
	for i := range metas {
		byID[metas[i].LibraryFileID] = metas[i]
	}
	out := make([]UserAppDTO, 0, len(files))
	for i := range files {
		f := files[i]
		dto := UserAppDTO{
			FileID:      f.FileID,
			AppName:     f.Name,
			SizeBytes:   f.SizeBytes,
			ParseStatus: ParseStatusParsing, // 尚未 finalize 的文件默认「解析中」
		}
		if m, ok := byID[f.FileID]; ok {
			dto.AppName = firstNonEmpty(m.AppName, f.Name)
			dto.PackageName = m.PackageName
			dto.Version = m.Version
			dto.IconURL = m.IconURL
			dto.ParseStatus = m.ParseStatus
			dto.ParseError = m.ParseError
			dto.CreatedAt = fmtTime(m.CreatedAt)
		} else {
			// 尚无 meta（经「素材」上传 / 直传等未显式 finalize 的入口，或历史卡在「解析中」）
			// → 异步触发一次解析使其自愈；本次仍返回「解析中」，前端轮询后收敛。
			s.ensureFinalized(userID, f.FileID)
		}
		out = append(out, dto)
	}
	return out, nil
}

// finalizingApps 去重在途的异步 finalize（key=fileID），避免列表轮询期间对同一文件重复触发解析。
var finalizingApps sync.Map

// ensureFinalized 对「尚无 app_user_meta」的 app 文件异步触发一次解析使其自愈：
// 覆盖经「素材」上传 / 直传等未显式调 finalize 的入口，以及历史卡在「解析中」的文件。
// 解析成功落 ready（含图标），失败落 failed —— 两者都会写出 meta，下次列表不再触发。
// 仅 OpenFileContent 阶段的瞬时错误（如对象未就绪）不写 meta，允许下次列表重试。
func (s *serviceImpl) ensureFinalized(userID int, fileID uint) {
	if _, inflight := finalizingApps.LoadOrStore(fileID, struct{}{}); inflight {
		return
	}
	go func() {
		defer finalizingApps.Delete(fileID)
		_, _ = s.FinalizeUserApp(userID, fileID)
	}()
}

// maxAppFileBytes 单个应用文件（apk/xapk）可解析的大小上限，防超大文件解析打爆内存/磁盘（O3）。
const maxAppFileBytes int64 = 1 << 30 // 1 GiB

// FinalizeUserApp 回读素材库对象 → 解析元数据/图标 → 落 app_user_meta（ready/failed），返回该应用 DTO。
func (s *serviceImpl) FinalizeUserApp(userID int, fileID uint) (*UserAppDTO, error) {
	content, err := library.OpenFileContent(userID, fileID)
	if err != nil {
		return nil, err // 属主/锁定/不存在/未配置 → 透传 library 门面错误
	}
	defer content.Reader.Close()

	// 大小门控：超大文件解析（zip 解压/图标解码）易打爆内存/磁盘，早拒（O3）。
	if content.SizeBytes > maxAppFileBytes {
		return nil, apperr.Validation("应用文件过大，无法解析")
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(content.Name), "."))
	tmp, err := os.CreateTemp("", "appfinalize-*."+ext)
	if err != nil {
		return nil, apperr.Internal("创建临时文件失败")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	// LimitReader 兜底：即便 content.Size 元数据谎报，也不会把超上限字节写进临时文件。
	if _, err := io.Copy(tmp, io.LimitReader(content.Reader, maxAppFileBytes)); err != nil {
		_ = tmp.Close()
		return nil, apperr.Internal("回读文件失败")
	}
	_ = tmp.Close()

	meta := &AppUserMeta{
		LibraryFileID: fileID,
		UserID:        uint(userID),
		AppName:       content.Name,
	}
	parsed, perr := apkparse.Parse(tmpPath, ext)
	if perr != nil {
		meta.ParseStatus = ParseStatusFailed
		meta.ParseError = truncate(perr.Error(), 512)
		if err := s.repo.upsertUserMeta(meta); err != nil {
			return nil, err
		}
		return s.userAppDTO(fileID, content.Name, content.SizeBytes), nil
	}

	iconURL, err := s.uploadIcon(parsed.IconPNG)
	if err != nil {
		return nil, err
	}
	meta.AppName = firstNonEmpty(parsed.AppName, content.Name)
	meta.PackageName = parsed.PackageName
	meta.Version = parsed.Version
	meta.IconURL = iconURL
	meta.MD5 = parsed.MD5
	meta.ParseStatus = ParseStatusReady
	meta.ParseError = ""
	if err := s.repo.upsertUserMeta(meta); err != nil {
		return nil, err
	}
	return s.userAppDTO(fileID, content.Name, content.SizeBytes), nil
}

// userAppDTO 取最新 meta 组装 DTO（finalize 后回读自身）。
func (s *serviceImpl) userAppDTO(fileID uint, fallbackName string, size int64) *UserAppDTO {
	dto := &UserAppDTO{FileID: fileID, AppName: fallbackName, SizeBytes: size, ParseStatus: ParseStatusParsing}
	if m, _ := s.repo.getUserMeta(fileID); m != nil {
		dto.AppName = firstNonEmpty(m.AppName, fallbackName)
		dto.PackageName = m.PackageName
		dto.Version = m.Version
		dto.IconURL = m.IconURL
		dto.ParseStatus = m.ParseStatus
		dto.ParseError = m.ParseError
		dto.CreatedAt = fmtTime(m.CreatedAt)
	}
	return dto
}

// BatchDeleteUserApps 删除当前用户的应用：逐个 library 删文件（释放配额）+ 删 meta。
func (s *serviceImpl) BatchDeleteUserApps(userID int, fileIDs []uint) error {
	if len(fileIDs) == 0 {
		return apperr.BadRequest("未选择应用")
	}
	for _, id := range fileIDs {
		if err := library.DeleteFileForUser(userID, id); err != nil {
			return err
		}
		if err := s.repo.deleteUserMeta(id); err != nil {
			return err
		}
	}
	return nil
}

// ---- 运营治理 ----

// AdminListUserApps 跨用户列出用户应用文件 + 上传者 + 解析元数据（按 meta.user_id 定属主）。
// 列表以 app_user_meta 为权威集合（已 finalize 的用户应用都有 meta），再补上传者信息。
func (s *serviceImpl) AdminListUserApps() ([]AdminUserAppDTO, error) {
	all, err := s.repo.listAllUserMeta()
	if err != nil {
		return nil, err
	}
	userIDs := make([]uint, 0, len(all))
	seen := map[uint]bool{}
	for i := range all {
		if !seen[all[i].UserID] {
			seen[all[i].UserID] = true
			userIDs = append(userIDs, all[i].UserID)
		}
	}
	uploaders, err := s.repo.listUploaders(userIDs)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint]uploaderInfo, len(uploaders))
	for i := range uploaders {
		byUser[uploaders[i].UserID] = uploaders[i]
	}
	out := make([]AdminUserAppDTO, 0, len(all))
	for i := range all {
		m := all[i]
		dto := AdminUserAppDTO{
			UserAppDTO: UserAppDTO{
				FileID:      m.LibraryFileID,
				AppName:     m.AppName,
				PackageName: m.PackageName,
				Version:     m.Version,
				IconURL:     m.IconURL,
				ParseStatus: m.ParseStatus,
				ParseError:  m.ParseError,
				CreatedAt:   fmtTime(m.CreatedAt),
			},
			UserID: m.UserID,
		}
		if u, ok := byUser[m.UserID]; ok {
			dto.UserPhone = u.UserPhone
			dto.UserNickname = u.UserNickname
		}
		out = append(out, dto)
	}
	return out, nil
}

// AdminBatchDeleteUserApps 跨用户删除：按 meta.user_id 定属主 → library 删文件 + 删 meta。
func (s *serviceImpl) AdminBatchDeleteUserApps(fileIDs []uint) error {
	if len(fileIDs) == 0 {
		return apperr.BadRequest("未选择应用")
	}
	for _, id := range fileIDs {
		m, err := s.repo.getUserMeta(id)
		if err != nil {
			return err
		}
		if m == nil {
			continue // 无 meta 无法定属主，跳过（library 删除需属主）
		}
		if err := library.DeleteFileForUser(int(m.UserID), id); err != nil {
			return err
		}
		if err := s.repo.deleteUserMeta(id); err != nil {
			return err
		}
	}
	return nil
}

// ---- 应用市场 ----

// ListMarket 列出应用市场（readyOnly=true 仅就绪，供用户端浏览）。
func (s *serviceImpl) ListMarket(readyOnly bool) ([]MarketAppDTO, error) {
	rows, err := s.repo.listMarket(readyOnly)
	if err != nil {
		return nil, err
	}
	out := make([]MarketAppDTO, 0, len(rows))
	for i := range rows {
		out = append(out, marketDTO(rows[i]))
	}
	return out, nil
}

func marketDTO(a AppMarket) MarketAppDTO {
	return MarketAppDTO{
		ID:          a.ID,
		AppName:     a.AppName,
		PackageName: a.PackageName,
		Version:     a.Version,
		IconURL:     a.IconURL,
		SizeBytes:   a.FileSize,
		ParseStatus: a.ParseStatus,
		ParseError:  a.ParseError,
		CreatedAt:   fmtTime(a.CreatedAt),
	}
}

// MarketUpload admin 上传市场应用：临时文件 → 解析 → PutObject 公有桶 + 图标 → 落 app_market。
func (s *serviceImpl) MarketUpload(tmpPath, ext string) (*MarketAppDTO, error) {
	if framework.S3 == nil {
		return nil, apperr.Internal("平台对象存储未配置")
	}
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext != "apk" && ext != "xapk" {
		return nil, apperr.BadRequest("仅支持 apk / xapk")
	}

	rec := &AppMarket{}
	parsed, perr := apkparse.Parse(tmpPath, ext)
	if perr != nil {
		// 解析失败仍落一行 failed 记录；S3Key 有唯一索引，给个占位键避免多次失败上传撞唯一约束。
		rec.S3Key = "parse-failed/" + uuid.NewString()
		rec.ParseStatus = ParseStatusFailed
		rec.ParseError = truncate(perr.Error(), 512)
		if err := s.repo.createMarket(rec); err != nil {
			return nil, err
		}
		dto := marketDTO(*rec)
		return &dto, nil
	}

	// 上传二进制到公有桶 app-market/<uuid>.<ext>。
	key := "app-market/" + uuid.NewString() + "." + ext
	f, err := os.Open(tmpPath)
	if err != nil {
		return nil, apperr.Internal("读取上传文件失败")
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	if err := framework.S3.PutObject(ctx, key, f, "application/vnd.android.package-archive"); err != nil {
		return nil, apperr.Internal("上传应用文件失败")
	}
	iconURL, err := s.uploadIcon(parsed.IconPNG)
	if err != nil {
		return nil, err
	}
	rec = &AppMarket{
		S3Key:       key,
		AppName:     firstNonEmpty(parsed.AppName, "应用"),
		PackageName: parsed.PackageName,
		Version:     parsed.Version,
		IconURL:     iconURL,
		MD5:         parsed.MD5,
		FileSize:    parsed.SizeBytes,
		ParseStatus: ParseStatusReady,
	}
	if err := s.repo.createMarket(rec); err != nil {
		return nil, err
	}
	dto := marketDTO(*rec)
	return &dto, nil
}

// MarketBatchDelete 删除市场应用：删公有桶对象 + 行。
func (s *serviceImpl) MarketBatchDelete(ids []uint) error {
	if len(ids) == 0 {
		return apperr.BadRequest("未选择应用")
	}
	rows, err := s.repo.getMarketByIDs(ids)
	if err != nil {
		return err
	}
	if framework.S3 != nil {
		ctx, cancel := opCtx()
		defer cancel()
		for i := range rows {
			if rows[i].S3Key != "" {
				_ = framework.S3.DeleteObject(ctx, rows[i].S3Key)
			}
		}
	}
	return s.repo.deleteMarketByIDs(ids)
}

// ---- 安装载荷解析（§7.2）----

// AppRef / InstallSpec 是安装载荷解析的入参/出参，由公开门面 app.go 同名结构按字段转换暴露。
type AppRef struct {
	Source string
	ID     uint
}

type InstallSpec struct {
	AppName     string
	DownloadURL string
	MD5         string
	PackageName string
	Version     string
	FileSize    string
}

// ResolveInstallSpecs 把应用引用解析为按 URL 安装载荷：
//   - user：校验 app_user_meta.parse_status=ready → library.OpenFileWithTTL(installTTL) 取 presigned GET。
//   - market：取 app_market(ready) 行 → DownloadURL = PublicBaseURL + "/" + S3Key。
//
// 任一未就绪/不存在/锁定 → 整请求失败。
func (s *serviceImpl) ResolveInstallSpecs(userID int, refs []AppRef) ([]InstallSpec, error) {
	if len(refs) == 0 {
		return nil, apperr.BadRequest("未选择应用")
	}
	out := make([]InstallSpec, 0, len(refs))
	for _, ref := range refs {
		switch ref.Source {
		case "user":
			m, err := s.repo.getUserMeta(ref.ID)
			if err != nil {
				return nil, err
			}
			if m == nil {
				return nil, apperr.NotFound("应用不存在")
			}
			if m.ParseStatus != ParseStatusReady {
				return nil, apperr.BadRequest("应用尚未就绪，无法安装")
			}
			fr, err := library.OpenFileWithTTL(userID, ref.ID, installTTL())
			if err != nil {
				return nil, err // 属主/锁定/不存在 → 透传
			}
			out = append(out, InstallSpec{
				AppName:     m.AppName,
				DownloadURL: fr.URL,
				MD5:         m.MD5,
				PackageName: m.PackageName,
				Version:     m.Version,
				FileSize:    strconv.FormatInt(fr.SizeBytes, 10),
			})
		case "market":
			a, err := s.repo.getMarketByID(ref.ID)
			if err != nil {
				return nil, err
			}
			if a == nil {
				return nil, apperr.NotFound("应用不存在")
			}
			if a.ParseStatus != ParseStatusReady {
				return nil, apperr.BadRequest("应用尚未就绪，无法安装")
			}
			downloadURL := a.S3Key
			if framework.S3 != nil {
				downloadURL = framework.S3.PublicURL(a.S3Key) // = PublicBaseURL + "/" + S3Key
			}
			out = append(out, InstallSpec{
				AppName:     a.AppName,
				DownloadURL: downloadURL,
				MD5:         a.MD5,
				PackageName: a.PackageName,
				Version:     a.Version,
				FileSize:    strconv.FormatInt(a.FileSize, 10),
			})
		default:
			return nil, apperr.BadRequest("未知应用来源")
		}
	}
	return out, nil
}

// ---- helpers ----

// uploadIcon 把解析出的图标 PNG 上传公有桶 app-icons/<uuid>.png，返回 IconURL。图标为空则返回空串。
func (s *serviceImpl) uploadIcon(png []byte) (string, error) {
	if len(png) == 0 {
		return "", nil
	}
	if framework.S3 == nil {
		return "", nil // 无公有桶时容忍无图标，不阻断解析落库
	}
	key := "app-icons/" + uuid.NewString() + ".png"
	ctx, cancel := opCtx()
	defer cancel()
	if err := framework.S3.PutObject(ctx, key, bytes.NewReader(png), "image/png"); err != nil {
		return "", apperr.Internal("上传图标失败")
	}
	return framework.S3.PublicURL(key), nil // = PublicBaseURL + "/" + key
}

// firstNonEmpty 返回第一个非空白字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
