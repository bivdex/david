package main

import (
	"fmt"
	"regexp"
	"strings"
)

// 正则表达式：前3位是字母或数字，中间3位是...，后4位是字母或数字
var pattern = regexp.MustCompile(`^[a-zA-Z0-9]{3}\.\.\.[a-zA-Z0-9]{4}$`)

// processString 校验字符串格式，替换中间的...为33位x，并添加0x前缀
func processString(s string) (string, bool) {
	// 校验格式是否符合要求
	if !pattern.MatchString(s) {
		return s, false
	}

	// 提取前3位和后4位
	prefix := s[:3]
	suffix := s[len(s)-4:]

	// 生成33位x的中间部分
	middle := strings.Repeat("x", 33)

	// 拼接结果并添加0x前缀
	result := "0x" + prefix + middle + suffix
	return result, true
}

func main() {
	// 测试用例
	testCases := []string{
		"59c...f028",  // 符合条件
		"a1b...c3d4",  // 符合条件
		"xyz...1234",  // 符合条件
		"ab...cdef",   // 前2位，不符合
		"abcd...efg",  // 前4位，不符合
		"123...456",   // 后3位，不符合
		"123..4567",   // 中间2个点，不符合
		"123....4567", // 中间4个点，不符合
		"@#$...1234",  // 前3位含特殊字符，不符合
		"123...$%^&",  // 后4位含特殊字符，不符合
	}

	for _, input := range testCases {
		output, valid := processString(input)
		status := "有效"
		if !valid {
			status = "无效"
		}
		fmt.Printf("输入: %-12s 输出: %-42s 状态: %s\n", input, output, status)
	}
}
