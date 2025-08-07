package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bivdex/david/eth-addr-generator/pkg/config"
	"github.com/bivdex/david/eth-addr-generator/pkg/database"
	"github.com/bivdex/david/eth-addr-generator/pkg/executor"
	"github.com/bivdex/david/eth-addr-generator/pkg/verify"
)

// 正则表达式：前3位是字母或数字，中间3位是...，后4位是字母或数字
var pattern = regexp.MustCompile(`^[a-fA-F0-9]{3}\.\.\.[a-fA-F0-9]{4}$`)

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

func writeWithWriteFile(lines []string, filename string) error {
	// 用换行符连接所有字符串
	content := strings.Join(lines, "\n")
	// 写入文件（0644表示读写权限）
	return os.WriteFile(filename, []byte(content), 0644)
}

// DBData 定义存储数据库行数据的结构体，清晰包含所需字段
type DBData struct {
	ID              interface{} // id字段（bigint类型）
	FromAddressPart string      // from_address_part字段
	AddressMask     string      //补充0x和xxx后的42位字符
	PrivateKey      string
	PubAddress      string
}

// 解析数据并构造参数和map，返回任务数据列表
func parseData(data []map[string]interface{}) ([]executor.TaskData, error) {
	var taskDataList []executor.TaskData

	for i, row := range data {
		// 提取id字段（bigint类型）
		id, hasID := row["id"]
		if !hasID {
			return nil, fmt.Errorf("第%d行数据缺少id字段", i)
		}

		// 提取from_address_part字段
		fromAddrVal, hasFromAddr := row["from_address_part"]
		if !hasFromAddr {
			log.Printf("第%d行数据缺少from_address_part字段，跳过该行", i)
			continue
		}

		// 处理from_address_part生成第三方程序参数
		fromAddrPart := convertToString(fromAddrVal)
		processRet, isValid := processString(convertToString(fromAddrVal))
		if isValid {
			// 创建任务数据
			taskData := executor.TaskData{
				TaskID:          i, // 使用索引作为taskid
				ID:              id,
				FromAddressPart: fromAddrPart,
				AddressMask:     processRet,
				PrivateKey:      "",
				PubAddress:      "",
			}
			taskDataList = append(taskDataList, taskData)
		} else {
			log.Printf("第%d行from_address_part处理无效: %s不符合格式要求", i+1, fromAddrPart)
		}
	}

	return taskDataList, nil
}

// 辅助函数：将任意类型转换为字符串（处理数据库可能的返回类型）
func convertToString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v) // 兜底转换
	}
}

// RowData 存储单行多列数据
type RowData struct {
	Columns []string // 列数据切片
	LineNum int      // 行号（用于错误提示）
}

// ReadMultiColumnFile 读取多列CSV文件
// minCols: 最小列数限制（0表示不限制）
// trimSpace: 是否自动去除每个字段的前后空格
func ReadMultiColumnFile(filename string, minCols int, trimSpace bool) ([]RowData, error) {
	// 打开文件
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("文件打开失败: %w", err)
	}
	defer file.Close()

	var rows []RowData
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	// 逐行读取
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// 处理空行（跳过）
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 按逗号分割字段
		columns := strings.Split(line, ",")

		// 处理字段前后空格
		if trimSpace {
			for i, col := range columns {
				columns[i] = strings.TrimSpace(col)
			}
		}

		// 验证最小列数
		if minCols > 0 && len(columns) < minCols {
			return rows, fmt.Errorf("第 %d 行列数不足（最少需要 %d 列，实际 %d 列）",
				lineNumber, minCols, len(columns))
		}

		// 添加到结果集
		rows = append(rows, RowData{
			Columns: columns,
			LineNum: lineNumber,
		})
	}

	// 检查扫描错误
	if err := scanner.Err(); err != nil {
		return rows, fmt.Errorf("文件读取错误: %w", err)
	}

	return rows, nil
}

// GenerateMatcher 根据公钥生成前 3 后 4 的匹配串（格式：前 3 位... 后 4 位）
// 处理逻辑：
// 1. 移除公钥可能包含的 "0x" 前缀
// 2. 提取处理后字符串的前 3 位和后 4 位
// 3. 拼接为 "前 3 位... 后 4 位" 格式
func GenerateMatcher(publicKey string) (string, error) {
	// 步骤 1：去除可能的 0x 前缀（不区分大小写）
	processedKey := strings.TrimPrefix(strings.ToLower(publicKey), "0x")

	// 验证处理后的公钥长度是否足够
	minLength := 7 // 前 3 位 + 后 4 位 = 至少 7 位
	if len(processedKey) < minLength {
		return "", errors.New(fmt.Sprintf(" 公钥长度不足，处理后长度为 % d，至少需要 % d 位 ", len(processedKey), minLength))
	}

	// 步骤 2：提取前 3 位和后 4 位
	prefix := processedKey[:3]
	suffix := processedKey[len(processedKey)-4:]

	// 步骤 3：拼接成 "前 3... 后 4" 格式
	return fmt.Sprintf("% s...% s", prefix, suffix), nil
}

// 在 config.yaml 的 app 配置中增加 loop_interval 字段（单位：秒）
// 例如：loop_interval: 60

// 工具函数：保存和读取 last_id
func SaveLastID(id int64, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("%d", id))
	return err
}

func LoadLastID(filename string) (int64, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // 文件不存在，默认从0开始
		}
		return 0, err
	}
	var id int64
	_, err = fmt.Sscanf(string(b), "%d", &id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// 工具函数：解析id为int64
func parseID(val interface{}) int64 {
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		var id int64
		fmt.Sscanf(v, "%d", &id)
		return id
	case []byte:
		var id int64
		fmt.Sscanf(string(v), "%d", &id)
		return id
	default:
		return 0
	}
}

// Application 应用程序结构体，包含所有依赖
type Application struct {
	config     *config.Config
	dbClient   *database.MySQLClient
	executor   *executor.Executor
	lastIDFile string
}

// NewApplication 创建新的应用程序实例
func NewApplication(configPath string) (*Application, error) {
	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 创建数据库客户端
	dbClient, err := database.NewMySQLClient(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to create database client: %w", err)
	}

	// 创建执行器
	exec := executor.NewExecutor(cfg.Executor)

	return &Application{
		config:     cfg,
		dbClient:   dbClient,
		executor:   exec,
		lastIDFile: "last_id.txt",
	}, nil
}

// Close 关闭应用程序资源
func (app *Application) Close() {
	if app.dbClient != nil {
		app.dbClient.Close()
	}
}

// queryData 查询数据库数据
func (app *Application) queryData(lastID int64) ([]map[string]interface{}, error) {
	cond := "id >= " + fmt.Sprintf("%d", lastID)
	if app.config.App.QueryCondition != "" {
		cond = fmt.Sprintf("(%s) AND %s", app.config.App.QueryCondition, cond)
	}
	if app.config.App.QueryCondition != "" {
		// 使用条件查询
		return app.dbClient.QueryDataWithCondition(
			app.config.App.TableName,
			cond,
			app.config.App.QueryLimit,
		)
	}
	// 查询所有数据
	return app.dbClient.QueryData(app.config.App.TableName, app.config.App.QueryLimit)
}

// processInputData 处理输入数据并写入文件，为每个任务生成独立的input文件
func (app *Application) processInputData(taskDataList []executor.TaskData) error {
	for _, taskData := range taskDataList {
		// 为每个任务生成独立的input文件
		inputFileName := fmt.Sprintf("input-%d.txt", taskData.TaskID)
		inputContent := []string{taskData.AddressMask}

		if err := writeWithWriteFile(inputContent, inputFileName); err != nil {
			return fmt.Errorf("failed to write %s: %w", inputFileName, err)
		}
		log.Printf("[任务%d] %s 写入成功", taskData.TaskID, inputFileName)
	}

	return nil
}

// executeCommands 执行第三方程序命令，为每个任务传递对应的文件名
func (app *Application) executeCommands(taskDataList []executor.TaskData) ([]*executor.ExecutionResult, error) {
	if app.config.App.EnableBatchMode {
		// 批量并发执行，传递最大任务数
		log.Printf("Starting batch execution with %d concurrent workers, max tasks: %d", app.config.App.MaxConcurrency, app.config.App.MaxTasks)
		return app.executor.ExecuteBatchWithTaskData(taskDataList, app.config.App.MaxConcurrency, app.config.App.MaxTasks)
	}

	// 顺序执行，受max_tasks限制
	log.Printf("Starting sequential execution with max tasks: %d", app.config.App.MaxTasks)
	results := make([]*executor.ExecutionResult, 0, len(taskDataList))
	maxTasks := app.config.App.MaxTasks
	if maxTasks <= 0 {
		maxTasks = 1
	}
	taskSem := make(chan struct{}, maxTasks)
	done := make(chan struct{})

	for _, taskData := range taskDataList {
		taskSem <- struct{}{} // acquire
		go func(td executor.TaskData) {
			defer func() { <-taskSem; done <- struct{}{} }() // release

			log.Printf("[任务%d] 开始执行第三方程序", td.TaskID)

			// 为每个任务传递对应的文件名参数
			params := map[string]interface{}{
				"input_file":  fmt.Sprintf("input-%d.txt", td.TaskID),
				"output_file": fmt.Sprintf("output-%d.txt", td.TaskID),
			}

			result, err := app.executor.Execute(params)
			if err != nil {
				log.Printf("[任务%d] Failed to execute command: %v", td.TaskID, err)
				result = &executor.ExecutionResult{
					Success:   false,
					Error:     err.Error(),
					Timestamp: time.Now(),
				}
			} else {
				log.Printf("[任务%d] 第三方程序执行完成", td.TaskID)
			}
			results = append(results, result)
		}(taskData)
	}
	// 等待所有任务完成
	for i := 0; i < len(taskDataList); i++ {
		<-done
	}
	return results, nil
}

// processResults 处理执行结果，读取每个任务的output文件
func (app *Application) processResults(taskDataList []executor.TaskData) error {
	// 记录是否已经更新过db
	historyDBUpdate := make(map[string]bool)

	for _, taskData := range taskDataList {
		outputFileName := fmt.Sprintf("output-%d.txt", taskData.TaskID)

		log.Printf("[任务%d] 开始读取结果文件: %s", taskData.TaskID, outputFileName)

		// 读取每个任务的output文件
		rows, err := ReadMultiColumnFile(outputFileName, 2, true)
		if err != nil {
			log.Printf("[任务%d] Failed to read %s: %v", taskData.TaskID, outputFileName, err)
			continue
		}

		log.Printf("[任务%d] 成功读取%s %d 行数据", taskData.TaskID, outputFileName, len(rows))

		for _, row := range rows {
			log.Printf("[任务%d] 行 %d（%d 列）: %v", taskData.TaskID, row.LineNum, len(row.Columns), row.Columns)
			privateKey := row.Columns[0]
			publicKey := row.Columns[1]

			// 根据publicKey获取前3后4的匹配串
			fromPart, _ := GenerateMatcher(publicKey)
			if verify.IsValidPrivateKey(privateKey) && !historyDBUpdate[fromPart] {
				if err := app.updateDatabase(fromPart, privateKey, publicKey, taskData); err != nil {
					log.Printf("[任务%d] db回写处理失败: %s,%s,err:%v", taskData.TaskID, taskData.ID, fromPart, err)
				} else {
					historyDBUpdate[fromPart] = true
				}
			}
		}
	}

	return nil
}

// updateDatabase 更新数据库记录
func (app *Application) updateDatabase(fromPart, privateKey, publicKey string, taskData executor.TaskData) error {
	// 要更新的字段和值
	updateParams := map[string]interface{}{
		"address":            publicKey,
		"match_success_time": time.Now(),
		"private_address":    privateKey,
	}

	rowsAffected, err := app.dbClient.UpdateByID(
		app.config.App.TableName,
		"id",
		taskData.ID,
		updateParams,
	)

	if err != nil {
		return err
	}

	if rowsAffected > 0 {
		log.Printf("[任务%d] db回写处理成功，影响 %d 行", taskData.TaskID, rowsAffected)
	}

	return nil
}

// cleanupFiles 清理临时文件，删除所有input-*.txt和output-*.txt
func (app *Application) cleanupFiles(taskDataList []executor.TaskData) {
	for _, taskData := range taskDataList {
		// 删除input文件
		inputFileName := fmt.Sprintf("input-%d.txt", taskData.TaskID)
		err := os.Remove(inputFileName)
		if err != nil {
			log.Printf("[任务%d] 删除%s失败,%v", taskData.TaskID, inputFileName, err)
		} else {
			log.Printf("[任务%d] 删除%s成功", taskData.TaskID, inputFileName)
		}

		// 删除output文件
		outputFileName := fmt.Sprintf("output-%d.txt", taskData.TaskID)
		err = os.Remove(outputFileName)
		if err != nil {
			log.Printf("[任务%d] 删除%s失败,%v", taskData.TaskID, outputFileName, err)
		} else {
			log.Printf("[任务%d] 删除%s成功", taskData.TaskID, outputFileName)
		}
	}
}

// printStatistics 打印执行统计信息
func (app *Application) printStatistics(data []map[string]interface{}, results []*executor.ExecutionResult, executionTime time.Duration) {
	successCount := 0
	failureCount := 0
	totalDuration := time.Duration(0)

	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}
		totalDuration += result.Duration
	}

	// 输出统计信息
	log.Println("=== 本次任务执行结果统计 ===")
	log.Printf("总记录数: %d\n", len(data))
	log.Printf("成功执行: %d\n", successCount)
	log.Printf("执行失败: %d\n", failureCount)
	log.Printf("总执行时间: %v\n", executionTime)
	if len(results) > 0 {
		log.Printf("平均执行时间: %v\n", totalDuration/time.Duration(len(results)))
	}
	log.Printf("成功率: %.2f%%\n", float64(successCount)/float64(len(data))*100)
}

// runLoop 循环执行主流程，支持定时和last_id
func (app *Application) runLoop() error {
	interval := 60 // 默认60秒
	if app.config.App.LoopInterval > 0 {
		interval = app.config.App.LoopInterval
	}
	for {
		lastID, err := LoadLastID(app.lastIDFile)
		if err != nil {
			log.Printf("读取last_id失败: %v", err)
			lastID = 0
		}
		log.Printf("本次从id>=%d开始查询", lastID)
		// 查询数据
		data, err := app.queryData(lastID)
		if err != nil {
			log.Printf("查询数据失败: %v", err)
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		if len(data) == 0 {
			log.Println("无新数据，等待下次轮询...")
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		log.Printf("本次查询到%d条数据", len(data))

		// 解析数据为任务列表
		taskDataList, err := parseData(data)
		if err != nil {
			log.Printf("解析数据失败: %v", err)
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		log.Printf("成功解析出%d个任务", len(taskDataList))

		// 处理输入数据
		if err := app.processInputData(taskDataList); err != nil {
			log.Printf("处理输入数据失败: %v", err)
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		// 执行第三方程序
		startTime := time.Now()
		results, err := app.executeCommands(taskDataList)
		if err != nil {
			log.Printf("执行命令失败: %v", err)
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		// 处理执行结果
		if err := app.processResults(taskDataList); err != nil {
			log.Printf("处理结果失败: %v", err)
		}
		// 清理文件
		app.cleanupFiles(taskDataList)
		// 打印统计信息
		executionTime := time.Since(startTime)
		app.printStatistics(data, results, executionTime)
		// 记录本次最大id
		maxID := lastID
		for _, row := range data {
			if idVal, ok := row["id"]; ok {
				id := parseID(idVal)
				if id > maxID {
					maxID = id
				}
			}
		}
		if err := SaveLastID(maxID, app.lastIDFile); err != nil {
			log.Printf("保存last_id失败: %v", err)
		}
		log.Printf("本次处理结束，最大id=%d，等待%d秒...", maxID, interval)
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// main 入口，调用 runLoop
func main() {
	configPath := "configs/config.yaml"
	app, err := NewApplication(configPath)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()
	if err := app.runLoop(); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
