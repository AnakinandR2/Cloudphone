package phone

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// 云手机实例业务状态机（本地档案为准，worker 据中台实时状态收敛异步过渡）。
//
//	CREATING        创建中（已调中台 create，等待 provisioning + 自动开机完成；期间禁止任何操作）
//	CREATE_FAILED   创建失败（provisioning 超时/失败；仅可销毁）
//	CREATED         已创建未开机（needStart=false：中台创建完成且未开机后收敛于此；无中台降级也用此态）
//	STARTING        开机中（已调中台 开机，等待 NORMAL；期间禁止任何操作）
//	RUNNING         运行中（创建完成即到此态；可关机、可远控；不可销毁；需先停止）
//	STOPPING        关机中（已调中台 关机，等待中台 STOPPED；期间禁止任何操作）
//	STOPPED         已停止（开机失败或关机后；可开机、可销毁）
//	DESTROYING      销毁中（已调中台 destroy，等待中台确认实例消失；期间禁止任何操作；确认后删本地档案）
const (
	StatusCreating     = "CREATING"
	StatusCreateFailed = "CREATE_FAILED"
	StatusCreated      = "CREATED"
	StatusStarting     = "STARTING"
	StatusRunning      = "RUNNING"
	StatusStopping     = "STOPPING"
	StatusStopped      = "STOPPED"
	StatusDestroying   = "DESTROYING"
	// StatusRecycled 回收态：席位不足导致超额实例进回收站（强制关机 + 释放席位占用），
	// 保留数据待清理。回收态实例不参与 reconcile、不占席位、不可开机；超保留天数后清理硬删。
	StatusRecycled = "RECYCLED"
	// StatusUnknown 展示态：中台不可用/查不到该 cp 时的实时状态，不回退本地档案值。
	StatusUnknown = "UNKNOWN"
)

// 中台云手机实时状态（CloudPhoneEnum，经 batch-query-status 查询）。
// 注意：这套枚举与「服务器/VM 状态」(ONLINE/OFFLINE) 不是一套——云手机就绪是 NORMAL 不是 ONLINE。
const (
	MidplatReady     = "NORMAL"    // 已开机/就绪 —— 开机成功的收敛信号
	MidplatStopped   = "STOPPED"   // 已关机 —— 关机成功的收敛信号
	MidplatDestroyed = "DESTROYED" // 已销毁
)

// midplatFailedStatuses 是 worker 视为「创建/开机失败」的中台终态。
var midplatFailedStatuses = map[string]bool{
	"INIT_FAILED": true, // 初始化失败
	"DESTROYED":   true, // 已销毁
	"FAULTED":     true, // 异常
}

// isMidplatFailed 判断中台实时状态是否为失败终态。
func isMidplatFailed(status string) bool { return midplatFailedStatuses[status] }

// 异步任务类型与状态（worker 据此把云手机从过渡态收敛到稳定态）。
const (
	TaskTypeCreate  = "create"  // CREATING → RUNNING / CREATE_FAILED
	TaskTypeStart   = "start"   // STARTING → RUNNING / STOPPED
	TaskTypeStop    = "stop"    // STOPPING → STOPPED
	TaskTypeDestroy = "destroy" // DESTROYING → 中台确认消失后删本地档案
)

const (
	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskSucceeded = "succeeded"
	TaskTimeout   = "timeout"
)

// CloudPhone 云手机实例的业务档案。按属主隔离：每台归属一个前台用户。
type CloudPhone struct {
	ID     uint `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID uint `gorm:"not null;index:idx_cloud_phone_user" json:"user_id"`
	// CpID 中台云手机 ID（实例开通后回填，未开通时为空）。
	CpID    string `gorm:"type:varchar(64);index:idx_cloud_phone_cpid" json:"cp_id"`
	Name    string `gorm:"type:varchar(100);not null" json:"name"`
	Status  string `gorm:"type:varchar(20);not null;default:'CREATED'" json:"status"`
	VmID    string `gorm:"type:varchar(64)" json:"vm_id"`
	ImageID string `gorm:"type:varchar(64)" json:"image_id"`
	// ProxyID 绑定的代理 ID（0 = 未绑定；未绑代理不可开机，校验属后续阶段）。
	ProxyID uint   `gorm:"not null;default:0" json:"proxy_id"`
	Remark  string `gorm:"type:varchar(255)" json:"remark"`
	// Tags 云手机标签（多个）；DB 以 JSON 数组字符串存于 tags 列，API 输出为数组。
	TagsJSON string `gorm:"column:tags;type:varchar(1000)" json:"-"`
	Tags     []Tag  `gorm:"-" json:"tags"`
	// AdbEnabled 由列表富化从中台 §2.6 实时判定（adbToken 非空），不入库；仅列表/卡片标记用。
	AdbEnabled bool `gorm:"-" json:"adb_enabled"`
	// Rooted 由列表富化从中台 §2.6 实时判定（isRooted），不入库；供列表标记 + 前端选对的 root 开关动作。
	Rooted bool `gorm:"-" json:"rooted"`
	// RecycledAt 进回收站时间（status=RECYCLED 时有值），用于计算剩余清理天数。
	RecycledAt *time.Time `gorm:"index:idx_cloud_phone_recycled" json:"recycled_at,omitempty"`
	// RecycleReason 进回收站原因（如「席位不足，超额回收」）。
	RecycleReason string    `gorm:"type:varchar(255)" json:"recycle_reason,omitempty"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (CloudPhone) TableName() string { return "cloud_phones" }

// CpTask 异步任务追踪：中台 create/startOrShutdown 均为异步，worker 轮询中台实时
// 状态把云手机从过渡态（CREATING/STARTING）收敛到稳定态，超时则补偿到失败态。
type CpTask struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint   `gorm:"not null;index:idx_cp_task_user" json:"user_id"`
	CloudPhoneID uint   `gorm:"not null;index:idx_cp_task_phone" json:"cloud_phone_id"`
	CpID         string `gorm:"type:varchar(64)" json:"cp_id"`
	Type         string `gorm:"type:varchar(20);not null" json:"type"`
	// ExpectedState 期望收敛到的中台实时状态（如 ONLINE）。
	ExpectedState string    `gorm:"type:varchar(20)" json:"expected_state"`
	Status        string    `gorm:"type:varchar(20);not null;default:'pending';index:idx_cp_task_status" json:"status"`
	Deadline      time.Time `json:"deadline"`
	LastError     string    `gorm:"type:varchar(255)" json:"last_error"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (CpTask) TableName() string { return "cp_tasks" }

// CloudPhoneCreate 创建请求（前台用户为自己新增云手机档案）。
type CloudPhoneCreate struct {
	Name    string `json:"name" binding:"required" example:"我的云手机1"`
	ImageID string `json:"image_id" example:"img-android13"`
	ProxyID uint   `json:"proxy_id" example:"0"`
	Remark  string `json:"remark"`
}

// CloudPhoneUpdate 更新请求（字段留空/零值表示不更新）。
type CloudPhoneUpdate struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	ImageID string `json:"image_id"`
	ProxyID uint   `json:"proxy_id"`
	Remark  string `json:"remark"`
}

// AfterFind 把 DB 里的 tags(JSON 串) 解析为 Tags 数组，供 API 直接输出。
func (c *CloudPhone) AfterFind(_ *gorm.DB) error {
	c.Tags = parseTags(c.TagsJSON)
	return nil
}

// Tag 云手机标签：名称 + 颜色（颜色为预设色 key，由前端映射成样式）。
type Tag struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// parseTags 解析 tags JSON：优先按 [{name,color}]，兼容历史的纯字符串数组。
func parseTags(s string) []Tag {
	if s == "" {
		return []Tag{}
	}
	var tags []Tag
	if err := json.Unmarshal([]byte(s), &tags); err == nil {
		return tags
	}
	var names []string
	if err := json.Unmarshal([]byte(s), &names); err == nil {
		out := make([]Tag, 0, len(names))
		for _, n := range names {
			out = append(out, Tag{Name: n})
		}
		return out
	}
	return []Tag{}
}

// marshalTags 按名称去空白、去重后序列化为 JSON 数组串（保留各自颜色）。
func marshalTags(tags []Tag) string {
	seen := map[string]bool{}
	out := []Tag{}
	for _, x := range tags {
		x.Name = strings.TrimSpace(x.Name)
		if x.Name == "" || seen[x.Name] {
			continue
		}
		seen[x.Name] = true
		out = append(out, x)
	}
	b, _ := json.Marshal(out)
	return string(b)
}
