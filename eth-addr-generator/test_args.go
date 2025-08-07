package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/bivdex/david/eth-addr-generator/pkg/config"
	"github.com/bivdex/david/eth-addr-generator/pkg/executor"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建执行器
	exec := executor.NewExecutor(cfg.Executor)

	// 测试不同的prefix_count和suffix_count配置
	testCases := []struct {
		prefixCount int
		suffixCount int
		description string
	}{
		{3, 4, "前3后4模式（默认）"},
		{2, 3, "前2后3模式"},
		{5, 6, "前5后6模式"},
		{1, 2, "前1后2模式"},
	}

	for _, testCase := range testCases {
		fmt.Printf("\n=== 测试 %s ===\n", testCase.description)
		
		// 模拟参数
		params := map[string]interface{}{
			"input_file":   "test-input.txt",
			"output_file":  "test-output.txt",
			"prefix_count": testCase.prefixCount,
			"suffix_count": testCase.suffixCount,
		}

		// 打印原始配置参数
		fmt.Printf("原始配置参数: %v\n", cfg.Executor.Args)
		
		// 模拟参数替换过程
		args := make([]string, len(cfg.Executor.Args))
		copy(args, cfg.Executor.Args)
		
		// 模拟替换过程
		if inputFile, hasInput := params["input_file"]; hasInput {
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--matching" {
					args[i+1] = fmt.Sprintf("%v", inputFile)
					break
				}
			}
		}

		if outputFile, hasOutput := params["output_file"]; hasOutput {
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--output" {
					args[i+1] = fmt.Sprintf("%v", outputFile)
					break
				}
			}
		}

		if prefixCount, hasPrefixCount := params["prefix_count"]; hasPrefixCount {
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--prefix-count" {
					args[i+1] = fmt.Sprintf("%v", prefixCount)
					break
				}
			}
		}

		if suffixCount, hasSuffixCount := params["suffix_count"]; hasSuffixCount {
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--suffix-count" {
					args[i+1] = fmt.Sprintf("%v", suffixCount)
					break
				}
			}
		}

		fmt.Printf("替换后参数: %v\n", args)
		
		// 构建完整的命令字符串
		commandStr := strings.Join(args, " ")
		fmt.Printf("完整命令: %s %s\n", cfg.Executor.ProgramPath, commandStr)
	}

	fmt.Println("\n=== 参数替换测试完成 ===")
}
