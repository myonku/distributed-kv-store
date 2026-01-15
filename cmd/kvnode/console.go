package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"
)

// 用于支持命令行交互控制台
func startCommandConsole(
	ctx context.Context,
	cancel context.CancelFunc,
	executor CommandExecutor,
	initialCmds []string,
) {
	if executor == nil {
		executor = &AppCommandExecutor{}
	}
	if cancel == nil {
		cancel = func() {}
	}

	// 启动时的预置命令（例如 -cmd "help; members"）
	for _, raw := range initialCmds {
		cmd, ok := parseCommandLine(raw)
		if !ok {
			continue
		}
		_ = runOneCommand(ctx, cancel, executor, cmd)
	}

	// 监听标准输入，启动交互式命令控制台
	go func() {
		log.Printf("command console started (type 'help' for commands)")
		scanner := bufio.NewScanner(os.Stdin)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !scanner.Scan() {
				return
			}
			// 读取并解析输入的命令行
			line := strings.TrimSpace(scanner.Text())
			cmd, ok := parseCommandLine(line)
			if !ok {
				continue
			}
			if err := runOneCommand(ctx, cancel, executor, cmd); err != nil {
				log.Printf("%s", formatCommandError(cmd, err))
			}
		}
	}()
}

// 执行单条命令
func runOneCommand(
	ctx context.Context,
	cancel context.CancelFunc,
	executor CommandExecutor,
	cmd ParsedCommand,
) error {
	switch cmd.Name {
	case "help", "-h", "?":
		log.Printf("available commands:\n%s", executor.Help())
		return nil
	case "exit", "-q":
		cancel()
		return nil
	default:
		return executor.Execute(ctx, cmd)
	}
}
