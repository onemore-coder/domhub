package model

import "time"

// RecordTemplate 解析记录模板：多条记录组合，可一键下发到任意 Zone。
// 典型场景：新站点一键创建 A + www CNAME + MX + SPF/TXT。
type RecordTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Remark    string    `gorm:"size:255" json:"remark"`
	Items     string    `gorm:"type:text;not null" json:"items"` // JSON 数组：[{name,type,value,ttl,priority,line}]
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (RecordTemplate) TableName() string { return "record_templates" }

// TemplateItem 模板条目（Items 字段的 JSON 结构）。
type TemplateItem struct {
	Name     string `json:"name"`  // 主机记录：@ / www / 前缀
	Type     string `json:"type"`  // A / CNAME / MX / TXT ...
	Value    string `json:"value"` // 记录值；{zone} 占位符会替换为下发目标 Zone 名
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
	Line     string `json:"line"`
}
