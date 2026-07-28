package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(
			os.Stderr,
			"用法: go run ./cmd/agent \"你的问题\"\n",
		)
		os.Exit(1)
	}

	prompt := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "错误：问题不能为空")
		os.Exit(1)
	}

	fmt.Println("Go Pi Agent")
	fmt.Println("------------")
	fmt.Printf("用户问题：%s\n", prompt)
}
