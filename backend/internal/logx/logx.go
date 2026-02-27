package logx

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// Info 功能：输出统一格式的 INFO 日志。
// 参数/返回：module/action/msg 为必填字段；kv 为可选的 key/value 成对参数。
// 失败场景：无（内部降级忽略不合法 kv）。
// 副作用：写入标准 logger 输出。
func Info(module, action, msg string, kv ...any) {
	log.Print(format("INFO", module, action, msg, kv...))
}

// Warn 功能：输出统一格式的 WARN 日志。
// 参数/返回：module/action/msg 为必填字段；kv 为可选的 key/value 成对参数。
// 失败场景：无（内部降级忽略不合法 kv）。
// 副作用：写入标准 logger 输出。
func Warn(module, action, msg string, kv ...any) {
	log.Print(format("WARN", module, action, msg, kv...))
}

// Error 功能：输出统一格式的 ERROR 日志。
// 参数/返回：module/action/msg 为必填字段；kv 为可选的 key/value 成对参数。
// 失败场景：无（内部降级忽略不合法 kv）。
// 副作用：写入标准 logger 输出。
func Error(module, action, msg string, kv ...any) {
	log.Print(format("ERROR", module, action, msg, kv...))
}

func format(level, module, action, msg string, kv ...any) string {
	var b strings.Builder
	b.WriteString("level=")
	b.WriteString(level)
	b.WriteString(" module=")
	b.WriteString(module)
	b.WriteString(" action=")
	b.WriteString(action)
	b.WriteString(" msg=")
	b.WriteString(strconv.Quote(msg))

	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok || key == "" {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(formatValue(kv[i+1]))
	}

	return b.String()
}

func formatValue(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case fmt.Stringer:
		return strconv.Quote(x.String())
	default:
		return fmt.Sprint(v)
	}
}
