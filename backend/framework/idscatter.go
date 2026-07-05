package framework

import (
	"fmt"
	"math/rand"

	"gorm.io/gorm"
)

// ScatterNextID 让 table 的「下一行」自增主键在刚用掉的 curID 基础上额外随机跳 0..4，
// 使相邻主键 gap∈[1,5] 不规则跳跃，避免主键被顺序枚举遍历（订单号、用户 ID 等同一策略）。
//
// 前提：table 主键为 DB 原生自增——本行 ID 已由 DB 原子分配、绝不受本函数影响（无 SELECT
// MAX+rand 竞态、无需撞键重试）。本函数只把自增序列「事后」多推几步。
//
// best-effort：推进出错一律吞掉；并发交错时最多导致下一行 gap=1（不跳），需求可接受
// （并非每行都必须跳）。三种库各用其原生序列推进写法，且只向前、不回退。
//
// 安全：table 由调用方传编译期常量表名、步数为本地随机整型，无外部输入，无 SQL 注入面。
func ScatterNextID(db *gorm.DB, table string, curID uint) {
	steps := rand.Intn(5) // 额外跳 0..4（对齐既有「虚假自增」先例）
	if steps <= 0 {
		return
	}
	switch db.Dialector.Name() {
	case "sqlite":
		// AUTOINCREMENT 表：下一个 id = sqlite_sequence.seq + 1；相对前移，避免并发下回退。
		db.Exec("UPDATE sqlite_sequence SET seq = seq + ? WHERE name = ?", steps, table)
	case "mysql":
		// InnoDB：把 AUTO_INCREMENT 设为 curID+1+steps（DDL 只向前、≤当前值时被忽略，天然不回退）。
		db.Exec(fmt.Sprintf("ALTER TABLE `%s` AUTO_INCREMENT = %d", table, curID+1+uint(steps)))
	case "postgres":
		// 连续 nextval steps 次烧号，把序列相对前移（始终向前，安全）。
		seq := table + "_id_seq"
		for i := 0; i < steps; i++ {
			db.Exec(fmt.Sprintf("SELECT nextval('%s')", seq))
		}
	}
}
