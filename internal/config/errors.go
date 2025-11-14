package config

import "fmt"

// FieldError 提供字段路径与错误原因，便于 CLI 向用户反馈。
type FieldError struct {
	Field  string
	Reason string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

// newFieldError 创建包含字段路径与原因的 error，便于 CLI 定位。
func newFieldError(field, reason string) error {
	return FieldError{Field: field, Reason: reason}
}

// hubField 用于拼接 Hub 级字段路径，方便输出 Hub[xxx].Field 形式。
func hubField(name, field string) string {
	if name == "" {
		return fmt.Sprintf("Hub[].%s", field)
	}
	return fmt.Sprintf("Hub[%s].%s", name, field)
}
