package automation

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// 脚本本地状态（仅 enabled/disabled；中台模板 status 0/1 与此对齐）。
const (
	ScriptEnabled  = "enabled"
	ScriptDisabled = "disabled"
)

// 周期计划状态（本地维护；中台 §7 无 plan 单查接口，靠启停删操作驱动）。
const (
	PlanNotStarted = "NOT_STARTED"
	PlanEnabling   = "ENABLING"
	PlanPaused     = "PAUSED"
	PlanFinished   = "FINISHED"
)

// 任务终态（手册 §7.6.2 taskStatus 枚举）。
var terminalTaskStatus = map[string]bool{
	"COMPLETED": true,
	"FAILED":    true,
	"CANCELLED": true,
}

// 触发方式。
const (
	TriggerManual = "manual" // 一次性立即运行
	TriggerPlan   = "plan"   // 周期计划派生
)

// AutomationScript 是脚本库本地记录（仿应用市场：store 区分商店/用户，user_id 属主隔离）。
//   - store=true , user_id=0     → 运营上传的商店脚本，面向全体客户。
//   - store=false, user_id=owner → 客户自己上传/编写的脚本。
//
// 中台模板是租户全局资源、按 name 无强隔离；为可靠取回 scriptId，上传时用唯一 mid_name，
// 展示名 name 单独存（编辑=重传得新 scriptId）。lua_content 留存以便编辑器重开。
type AutomationScript struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserID      uint   `gorm:"index;not null" json:"-"`
	Store       bool   `gorm:"index;default:false" json:"store"`
	ScriptID    int64  `gorm:"index" json:"scriptId"`   // 中台 scriptId
	MidName     string `gorm:"size:128;index" json:"-"` // 中台模板唯一名
	Name        string `gorm:"size:255" json:"name"`    // 展示名
	Description string `gorm:"size:1024" json:"description"`
	Version     string `gorm:"size:64" json:"version"`
	LuaContent  string `gorm:"type:text" json:"luaContent"`
	// 参数 schema 不再单独存列：真源是 lua_content 顶部 --[[ ]] 注释，需要时现 parse。
	FileName  string    `gorm:"size:255" json:"fileName"`
	Status    string    `gorm:"size:16" json:"status"` // enabled / disabled
	CreatedAt time.Time `json:"createTime"`
	UpdatedAt time.Time `json:"updateTime"`
}

func (AutomationScript) TableName() string { return "automation_scripts" }

// AdminScript 给运营治理视图附带上传者 ID。
type AdminScript struct {
	AutomationScript
	UploaderID uint `json:"uploaderId"`
}

// AutomationPlan 是周期计划本地镜像（§7.3）。
type AutomationPlan struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index;not null" json:"-"`
	PlanID        int64     `gorm:"index" json:"planId"`           // 中台计划主键
	PlanUID       string    `gorm:"size:128;index" json:"planUid"` // 中台计划 UID
	ScriptLocalID uint      `gorm:"index" json:"scriptId"`
	ScriptName    string    `gorm:"size:255" json:"scriptName"`
	Name          string    `gorm:"size:255" json:"name"`
	Frequency     string    `gorm:"size:16" json:"frequency"` // INTERVAL / DAILY
	IntervalValue int       `json:"intervalValue"`
	ExecutionTime string    `gorm:"size:16" json:"executionTime"`
	StartTime     string    `gorm:"size:32" json:"startTime"`
	EndTime       string    `gorm:"size:32" json:"endTime"`
	CpIDsJSON     string    `gorm:"column:cp_ids;type:text" json:"-"`
	CpIDs         []string  `gorm:"-" json:"cpIds"`
	Status        string    `gorm:"size:16" json:"status"`
	CreatedAt     time.Time `json:"createTime"`
	UpdatedAt     time.Time `json:"updateTime"`
}

func (AutomationPlan) TableName() string { return "automation_plans" }

// AfterFind 把 cp_ids JSON 串解析为数组。
func (p *AutomationPlan) AfterFind(_ *gorm.DB) error {
	p.CpIDs = parseStrList(p.CpIDsJSON)
	return nil
}

// AutomationTask 是任务索引（一次性创建即插入；计划派生由 worker 发现 upsert）。供任务日志列表。
type AutomationTask struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index;not null" json:"-"`
	MidTaskID     int64     `gorm:"uniqueIndex" json:"midTaskId"`
	TaskNo        string    `gorm:"size:64" json:"taskNo"`
	ScriptLocalID uint      `gorm:"index" json:"scriptId"`
	ScriptName    string    `gorm:"size:255" json:"scriptName"`
	PlanLocalID   uint      `gorm:"index;default:0" json:"planId"` // 0=一次性
	CpID          string    `gorm:"size:64;index" json:"cpId"`
	TaskName      string    `gorm:"size:255" json:"taskName"`
	Trigger       string    `gorm:"size:16" json:"trigger"` // manual / plan
	LastStatus    string    `gorm:"size:24" json:"status"`  // 英文枚举（§7.6.2）
	RunStart      string    `gorm:"size:32" json:"runStart"`
	RunEnd        string    `gorm:"size:32" json:"runEnd"`
	CreatedAt     time.Time `json:"createTime"`
	UpdatedAt     time.Time `json:"updateTime"`
}

func (AutomationTask) TableName() string { return "automation_tasks" }

// TaskReport 是任务详情（状态 + 报告）的合并视图。
type TaskReport struct {
	MidTaskID     int64  `json:"midTaskId"`
	TaskNo        string `json:"taskNo"`
	Status        string `json:"status"`     // 英文枚举（§7.6.2）
	StatusDesc    string `json:"statusDesc"` // 中文描述
	Terminal      bool   `json:"terminal"`
	RunDurationMs int64  `json:"runDurationMs"`
	RunLog        string `json:"runLog"`
	Result        string `json:"result"`
	ScreenshotURL string `json:"screenshotUrl"`
}

// parseStrList 解析 JSON 字符串数组（容错空/非法）。
func parseStrList(s string) []string {
	if s == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	return out
}

// marshalStrList 序列化字符串数组。
func marshalStrList(list []string) string {
	b, _ := json.Marshal(list)
	return string(b)
}
