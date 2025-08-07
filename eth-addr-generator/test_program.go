package main

import (
	"fmt"
	"regexp"
	"strings"
)

// 生成动态正则表达式：前N位是字母或数字，中间是...，后M位是字母或数字
func generatePattern(prefixCount, suffixCount int) *regexp.Regexp {
	patternStr := fmt.Sprintf(`^[a-fA-F0-9]{%d}\.\.\.[a-fA-F0-9]{%d}$`, prefixCount, suffixCount)
	return regexp.MustCompile(patternStr)
}

// processString 校验字符串格式，替换中间的...为33位x，并添加0x前缀
// 支持动态的前N位后M位模式
func processString(s string, prefixCount, suffixCount int) (string, bool) {
	// 生成动态正则表达式
	pattern := generatePattern(prefixCount, suffixCount)

	// 校验格式是否符合要求
	if !pattern.MatchString(s) {
		return s, false
	}

	// 提取前N位和后M位
	prefix := s[:prefixCount]
	suffix := s[len(s)-suffixCount:]

	// 生成33位x的中间部分
	middle := strings.Repeat("x", 33)

	// 拼接结果并添加0x前缀
	result := "0x" + prefix + middle + suffix
	return result, true
}

func main() {
	// 测试前3后4模式（原有模式）
	fmt.Println("=== 测试前3后4模式 ===")
	testCases := []string{
		"abc...defg",  // 有效
		"123...4567",  // 有效
		"ab...defg",   // 无效：前缀不足3位
		"abc...def",   // 无效：后缀不足4位
		"abc..defg",   // 无效：中间不是...
		"abc...defgh", // 无效：后缀超过4位
	}

	for _, testCase := range testCases {
		result, valid := processString(testCase, 3, 4)
		fmt.Printf("输入: %s, 有效: %t, 输出: %s\n", testCase, valid, result)
	}

	// 测试前2后3模式（新模式）
	fmt.Println("\n=== 测试前2后3模式 ===")
	testCases2 := []string{
		"ab...def",  // 有效
		"12...345",  // 有效
		"a...def",   // 无效：前缀不足2位
		"ab...de",   // 无效：后缀不足3位
		"abc...def", // 无效：前缀超过2位
	}

	for _, testCase := range testCases2 {
		result, valid := processString(testCase, 2, 3)
		fmt.Printf("输入: %s, 有效: %t, 输出: %s\n", testCase, valid, result)
	}

	// 测试前5后6模式（新模式）
	fmt.Println("\n=== 测试前5后6模式 ===")
	testCases3 := []string{
		"abcde...123456", // 有效
		"12345...abcdef", // 有效
		"abcd...123456",  // 无效：前缀不足5位
		"abcde...12345",  // 无效：后缀不足6位
	}

	for _, testCase := range testCases3 {
		result, valid := processString(testCase, 5, 6)
		fmt.Printf("输入: %s, 有效: %t, 输出: %s\n", testCase, valid, result)
	}
}
