package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// 将GBK编码转换为UTF-8
func gbkToUtf8(gbkBytes []byte) string {
	reader := transform.NewReader(bytes.NewReader(gbkBytes), simplifiedchinese.GBK.NewDecoder())
	utf8Bytes, err := io.ReadAll(reader)
	if err != nil {
		return string(gbkBytes)
	}
	return string(utf8Bytes)
}

// 实时读取流并打印（支持编码转换）
func readAndPrint(stream io.ReadCloser, wg *sync.WaitGroup, isError bool) {
	defer wg.Done()
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Bytes()
		// 转换编码并打印
		if isError {
			log.Printf("错误输出: %s", gbkToUtf8(line))
		} else {
			log.Printf("输出: %s", gbkToUtf8(line))
		}
	}

	// 检查扫描错误
	if err := scanner.Err(); err != nil {
		log.Printf("读取流错误: %v", err)
	}
}

func main() {
	// 定义程序路径和参数
	exeName := "profanity.exe"
	args := []string{
		"--matching", "../input.txt",
		"--output", "result.txt",
		"--prefix-count", "3",
		"--suffix-count", "4",
		"--quit-count", "4",
	}

	// 获取程序绝对路径
	exePath, err := filepath.Abs(exeName)
	if err != nil {
		log.Fatalf("无法获取程序绝对路径: %v", err)
	}

	// 检查程序是否存在
	if _, err := os.Stat(exePath); err != nil {
		log.Fatalf("程序不存在: %s, 错误: %v", exePath, err)
	}

	log.Printf("开始执行: %s %v", exePath, args)

	// 创建命令
	cmd := exec.Command(exePath, args...)

	// 分别获取标准输出和错误输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("获取标准输出失败: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("获取错误输出失败: %v", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		log.Fatalf("启动命令失败: %v", err)
	}

	// 使用WaitGroup等待两个流读取完毕
	var wg sync.WaitGroup
	wg.Add(2)

	// 异步读取标准输出
	go readAndPrint(stdout, &wg, false)
	// 异步读取错误输出
	go readAndPrint(stderr, &wg, true)

	// 等待所有输出处理完毕
	wg.Wait()

	// 等待命令执行完成
	if err := cmd.Wait(); err != nil {
		log.Fatalf("命令执行失败: %v", err)
	}

	log.Println("程序执行完成，结果已保存到 result.txt")
}
