package example

import "time"

// ExampleItem 示例数据模型
type ExampleItem struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"type:varchar(100);not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ExampleItem) TableName() string { return "example_items" }

// ExampleItemCreate 创建请求
type ExampleItemCreate struct {
	Title   string `json:"title" binding:"required" example:"示例标题"`
	Content string `json:"content" example:"示例内容"`
}

// ExampleItemUpdate 更新请求
type ExampleItemUpdate struct {
	Title   string `json:"title" example:"新标题"`
	Content string `json:"content" example:"新内容"`
}
