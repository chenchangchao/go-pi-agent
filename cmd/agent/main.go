package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chenchangchao/go-pi-agent/internal/agenttool"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(
			os.Stderr,
			`用法: go run ./cmd/agent "文件路径"`,
		)
		os.Exit(1)
	}

	path := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if path == "" {
		fmt.Fprintln(os.Stderr, "错误：文件路径不能为空")
		os.Exit(1)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取当前目录失败：%v\n", err)
		os.Exit(1)
	}

	readFileTool, err := agenttool.NewReadFileTool(workingDirectory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 read_file 工具失败：%v\n", err)
		os.Exit(1)
	}

	arguments, err := json.Marshal(map[string]string{
		"path": path,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "编码参数失败：%v\n", err)
		os.Exit(1)
	}

	result, err := readFileTool.Execute(context.Background(), arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取文件失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Println("Go Pi Agent")
	fmt.Println("------------")
	fmt.Println(result)
}
