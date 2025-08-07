package main

import (
	"fmt"
	"regexp"
)

// generatePattern 生成正则表达式：前N位是字母或数字，中间是三个点(...), 后M位是字母或数字
// 参数:
//
//	prefixCount - 前缀的字符数量
//	suffixCount - 后缀的字符数量
//
// 返回:
//
//	编译好的正则表达式对象
func generatePattern(prefixCount, suffixCount int) *regexp.Regexp {
	// 使用 \.{3} 精确匹配三个点，比 \.\.\. 更简洁且不易出错
	// [a-zA-Z0-9] 匹配所有字母(大小写)和数字
	patternStr := fmt.Sprintf(`^[a-zA-Z0-9]{%d}\.{3}[a-zA-Z0-9]{%d}$`, prefixCount, suffixCount)
	return regexp.MustCompile(patternStr)
}

// 示例用法
func main() {
	// 生成匹配: 2位前缀 + ... + 3位后缀 的正则
	pattern := generatePattern(0, 1)

	testCases := []string{
		"ab...123",  // 匹配
		"a1...b2c",  // 匹配
		"ab..123",   // 不匹配 (只有两个点)
		"ab....123", // 不匹配 (四个点)
		"a...123",   // 不匹配 (前缀长度不够)
		"abc...12",  // 不匹配 (后缀长度不够)
		"...abc",
		"...1",
	}

	for _, test := range testCases {
		fmt.Printf("字符串: %-10s 匹配结果: %v\n", test, pattern.MatchString(test))
	}
}
