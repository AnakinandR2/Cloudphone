package note

import "time"

// Note 前台用户的笔记（按属主隔离：每条归属一个 user）。
type Note struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"not null;index:idx_note_user" json:"user_id"`
	Title     string    `gorm:"type:varchar(100);not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Note) TableName() string { return "notes" }

// NoteCreate 创建请求
type NoteCreate struct {
	Title   string `json:"title" binding:"required" example:"标题"`
	Content string `json:"content" example:"内容"`
}

// NoteUpdate 更新请求
type NoteUpdate struct {
	Title   string `json:"title" example:"新标题"`
	Content string `json:"content" example:"新内容"`
}
