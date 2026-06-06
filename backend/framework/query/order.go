// Package query 提供列表查询相关的通用助手。
package query

import "strings"

// SafeOrder 基于白名单构造安全的 ORDER BY 子句，杜绝 SQL 注入。
//
//	order   请求传入的排序字段（不可信）
//	sort    排序方向，"descending" 为降序，其余按升序
//	allowed 允许排序的字段白名单
//	def     字段非法或为空时回退的默认排序（如 "id DESC"）
//
// 仅当 order 命中白名单时才拼接，否则返回 def，因此 order 永远来自可信集合。
func SafeOrder(order, sort string, allowed map[string]bool, def string) string {
	if order == "" || !allowed[order] {
		return def
	}
	dir := "ASC"
	if strings.EqualFold(sort, "descending") {
		dir = "DESC"
	}
	return order + " " + dir
}
