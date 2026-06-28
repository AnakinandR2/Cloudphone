package framework

import "log"

// LogStructured 简易结构化日志：[evt] <event> k1=v1 k2=v2 ...
// 仅依赖标准库 log，零业务引用。后续换 slog/zap 只换实现、调用点不动。
func LogStructured(event string, kv ...string) {
	buf := []byte("[evt] " + event)
	for i := 0; i+1 < len(kv); i += 2 {
		buf = append(buf, ' ')
		buf = append(buf, kv[i]...)
		buf = append(buf, '=')
		buf = append(buf, kv[i+1]...)
	}
	log.Println(string(buf))
}
