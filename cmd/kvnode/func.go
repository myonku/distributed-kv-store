package main

import (
	"fmt"
	"strings"
)

// 解析命令行字符串，返回 ParsedCommand
func parseCommandLine(line string) (ParsedCommand, bool) {
	raw := strings.TrimSpace(line)
	if raw == "" {
		return ParsedCommand{}, false
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ParsedCommand{}, false
	}
	return ParsedCommand{Name: strings.ToLower(fields[0]), Args: fields[1:], Raw: raw}, true
}

// 格式化命令执行错误信息
func formatCommandError(cmd ParsedCommand, err error) string {
	if err == nil {
		return ""
	}
	if cmd.Raw != "" {
		return fmt.Sprintf("command %q failed: %v", cmd.Raw, err)
	}
	return fmt.Sprintf("command failed: %v", err)
}
