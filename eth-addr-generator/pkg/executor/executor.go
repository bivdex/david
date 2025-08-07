package executor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	ProgramPath string            `mapstructure:"program_path"`
	Args        []string          `mapstructure:"args"`
	Timeout     int               `mapstructure:"timeout"` // 超时时间（秒）
	WorkingDir  string            `mapstructure:"working_dir"`
	Env         map[string]string `mapstructure:"env"`
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Success   bool
	Output    string
	Error     string
	ExitCode  int
	Duration  time.Duration
	Timestamp time.Time
}

// Executor 第三方程序执行器
type Executor struct {
	config ExecutorConfig
}

// NewExecutor 创建执行器
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		config: config,
	}
}

// Execute 执行第三方程序
func (e *Executor) Execute(params map[string]interface{}) (*ExecutionResult, error) {
	startTime := time.Now()

	// 构建命令参数
	args := make([]string, len(e.config.Args))
	copy(args, e.config.Args)

	// 动态替换 --matching 和 --output 参数
	if inputFile, hasInput := params["input_file"]; hasInput {
		// 查找并替换 --matching 参数
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--matching" {
				args[i+1] = fmt.Sprintf("%v", inputFile)
				break
			}
		}
	}

	if outputFile, hasOutput := params["output_file"]; hasOutput {
		// 查找并替换 --output 参数
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--output" {
				args[i+1] = fmt.Sprintf("%v", outputFile)
				break
			}
		}
	}

	// 动态替换 --prefix-count 和 --suffix-count 参数
	if prefixCount, hasPrefixCount := params["prefix_count"]; hasPrefixCount {
		// 查找并替换 --prefix-count 参数
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--prefix-count" {
				args[i+1] = fmt.Sprintf("%v", prefixCount)
				break
			}
		}
	}

	if suffixCount, hasSuffixCount := params["suffix_count"]; hasSuffixCount {
		// 查找并替换 --suffix-count 参数
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--suffix-count" {
				args[i+1] = fmt.Sprintf("%v", suffixCount)
				break
			}
		}
	}

	// 将其他参数添加到命令中
	for key, value := range params {
		// 跳过已经处理的 input_file 和 output_file
		if key == "input_file" || key == "output_file" || key == "prefix_count" || key == "suffix_count" {
			continue
		}

		// 根据参数类型构建命令行参数
		switch v := value.(type) {
		case string:
			args = append(args, fmt.Sprintf("--%s=%s", key, v))
		case int, int64:
			args = append(args, fmt.Sprintf("--%s=%d", key, v))
		case float64:
			args = append(args, fmt.Sprintf("--%s=%f", key, v))
		case bool:
			if v {
				args = append(args, fmt.Sprintf("--%s", key))
			}
		default:
			args = append(args, fmt.Sprintf("--%s=%v", key, v))
		}
	}

	// 创建命令
	var cmd *exec.Cmd
	// 执行命令前先设置控制台编码为 UTF-8（Windows 特定）
	exec.Command("chcp", "65001").Run()

	if runtime.GOOS == "windows" {
		// 关键：使用绝对路径确保可执行文件能被找到
		exePath := filepath.Join(".", "external", e.config.ProgramPath)
		// 转换为绝对路径便于调试
		absPath, err := filepath.Abs(exePath)
		if err != nil {
			log.Fatalf("获取绝对路径失败: %v", err)
		}
		log.Printf("尝试执行的程序路径: %s", absPath)

		cmd = exec.Command(absPath, args...)
	} else {
		cmd = exec.Command(e.config.ProgramPath, args...)
	}

	// 设置工作目录
	if e.config.WorkingDir != "" {
		cmd.Dir = e.config.WorkingDir
	}

	// 设置环境变量
	if len(e.config.Env) > 0 {
		for key, value := range e.config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	//设置超时
	var timeout time.Duration
	if e.config.Timeout > 0 {
		timeout = time.Duration(e.config.Timeout) * time.Second
	} else {
		timeout = 30 * time.Second // 默认30秒超时
	}

	//创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	// 捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	err := cmd.Run()
	duration := time.Since(startTime)

	result := &ExecutionResult{
		Output:    strings.TrimSpace(stdout.String()),
		Error:     strings.TrimSpace(stderr.String()),
		Duration:  duration,
		Timestamp: time.Now(),
	}

	if err != nil {
		result.Success = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		log.Printf("Command execution failed: %v", err)
	} else {
		result.Success = true
		result.ExitCode = 0
		log.Printf("Command executed successfully in %v", duration)
	}

	log.Printf("Command: %s %s", e.config.ProgramPath, strings.Join(args, " "))
	log.Printf("Output: %s", result.Output)
	if result.Error != "" {
		log.Printf("Error: %s", result.Error)
	}

	return result, nil
}

// ExecuteWithData 使用数据库数据执行第三方程序
func (e *Executor) ExecuteWithData(data []map[string]interface{}) ([]*ExecutionResult, error) {
	var results []*ExecutionResult
	// 执行第三方程序
	var emptyMap map[string]interface{}
	result, err := e.Execute(emptyMap)
	if err != nil {
		log.Printf("Failed to execute command  %v", err)
		return nil, err
	} else {
		results = append(results, result)
	}

	return results, nil
}

// ExecuteBatch 批量执行，支持并发和最大任务数限制
func (e *Executor) ExecuteBatch(data []map[string]interface{}, maxConcurrency int, maxTasks int) ([]*ExecutionResult, error) {
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	if maxTasks <= 0 {
		maxTasks = 1
	}

	// 信号量，控制最大任务数
	taskSem := make(chan struct{}, maxTasks)

	// 创建任务通道
	tasks := make(chan map[string]interface{}, len(data))
	results := make(chan *ExecutionResult, len(data))

	// 启动工作协程
	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for task := range tasks {
				taskSem <- struct{}{} // acquire
				params := make(map[string]interface{})
				for key, value := range task {
					params[key] = value
				}
				result, err := e.Execute(params)
				if err != nil {
					log.Printf("Failed to execute command: %v", err)
					result = &ExecutionResult{
						Success:   false,
						Error:     err.Error(),
						Timestamp: time.Now(),
					}
				}
				results <- result
				<-taskSem // release
			}
		}()
	}

	// 发送任务
	for _, row := range data {
		tasks <- row
	}
	close(tasks)

	// 收集结果
	var allResults []*ExecutionResult
	for i := 0; i < len(data); i++ {
		result := <-results
		allResults = append(allResults, result)
	}

	return allResults, nil
}

// TaskData 定义单个任务的数据结构
type TaskData struct {
	TaskID          int         // 任务ID（索引）
	ID              interface{} // 数据库id字段
	FromAddressPart string      // from_address_part字段
	AddressMask     string      // 补充0x和xxx后的42位字符
	PrefixCount     int
	SuffixCount     int
}

// ExecuteBatchWithTaskData 批量执行，支持TaskData结构体和最大任务数限制
func (e *Executor) ExecuteBatchWithTaskData(taskDataList []TaskData, maxConcurrency int, maxTasks int) ([]*ExecutionResult, error) {
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	if maxTasks <= 0 {
		maxTasks = 1
	}

	// 信号量，控制最大任务数
	taskSem := make(chan struct{}, maxTasks)

	// 创建任务通道
	tasks := make(chan TaskData, len(taskDataList))
	results := make(chan *ExecutionResult, len(taskDataList))

	// 启动工作协程
	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for taskData := range tasks {
				taskSem <- struct{}{} // acquire

				log.Printf("[任务%d] 开始执行第三方程序", taskData.TaskID)

				// 为每个任务传递对应的文件名参数
				params := map[string]interface{}{
					"input_file":   fmt.Sprintf("input-%d.txt", taskData.TaskID),
					"output_file":  fmt.Sprintf("output-%d.txt", taskData.TaskID),
					"prefix_count": taskData.PrefixCount,
					"suffix_count": taskData.SuffixCount,
				}

				result, err := e.Execute(params)
				if err != nil {
					log.Printf("[任务%d] Failed to execute command: %v", taskData.TaskID, err)
					result = &ExecutionResult{
						Success:   false,
						Error:     err.Error(),
						Timestamp: time.Now(),
					}
				} else {
					log.Printf("[任务%d] 第三方程序执行完成", taskData.TaskID)
				}
				results <- result
				<-taskSem // release
			}
		}()
	}

	// 发送任务
	for _, taskData := range taskDataList {
		tasks <- taskData
	}
	close(tasks)

	// 收集结果
	var allResults []*ExecutionResult
	for i := 0; i < len(taskDataList); i++ {
		result := <-results
		allResults = append(allResults, result)
	}

	return allResults, nil
}
