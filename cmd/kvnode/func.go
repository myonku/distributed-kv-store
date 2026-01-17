package main

import (
	"flag"
	"fmt"
	"os"
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

// 解析启动参数
func parseStartupFlags(args []string) (configPath string, consoleEnabled bool, initialCmds []string) {
	fs := flag.NewFlagSet("kvnode", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	config := fs.String("config", "settings.toml", "path to settings.toml")
	console := fs.Bool("console", true, "enable interactive console")
	cmds := fs.String("cmd", "", "initial commands separated by ';'")

	_ = fs.Parse(args)

	var parsedCmds []string
	if *cmds != "" {
		for raw := range strings.SplitSeq(*cmds, ";") {
			c := strings.TrimSpace(raw)
			if c != "" {
				parsedCmds = append(parsedCmds, c)
			}
		}
	}

	return *config, *console, parsedCmds
}
