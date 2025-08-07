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

// 生成动态正则表达式：前N位是字母或数字，中间是...，后M位是字母或数字
func generatePattern(prefixCount, suffixCount int) *regexp.Regexp {
	patternStr := fmt.Sprintf(`^[a-fA-F0-9]{%d}\.\.\.[a-fA-F0-9]{%d}$`, prefixCount, suffixCount)
	return regexp.MustCompile(patternStr)
}

// getPrefixAndSuffixLengths 解析包含三个点的字符串，返回前缀和后缀的长度
// 参数:
//
//	s - 包含三个点(...)的字符串
//
// 返回:
//
//	prefixLen - 三个点前面的字符长度
//	suffixLen - 三个点后面的字符长度
//	err - 如果字符串格式不符合要求则返回错误
func getPrefixAndSuffixLengths(s string) (prefixLen, suffixLen int, err error) {
	// 使用正则表达式捕获三个点前后的内容
	re := regexp.MustCompile(`^([a-zA-Z0-9]*)\.{3}([a-zA-Z0-9]*)$`)
	matches := re.FindStringSubmatch(s)

	// 检查匹配结果
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("字符串格式不符合要求，必须包含三个连续的点(...)")
	}

	// 返回前后缀的长度
	return len(matches[1]), len(matches[2]), nil
}

// processString 校验字符串格式，替换中间的...为x，并添加0x前缀
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

	// 生成x的中间部分
	middle := strings.Repeat("x", 40-prefixCount-suffixCount)

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
		// 根据s中的...前后的字符串个数获取prefixCount suffixCount
		prefixCount, suffixCount, err := getPrefixAndSuffixLengths(fromAddrPart)
		if err != nil {
			log.Printf("第%d行数据from_address_part %s格式有错误，跳过该行", i, fromAddrPart)
			continue
		}
		processRet, isValid := processString(fromAddrPart, prefixCount, suffixCount)
		if isValid {
			// 创建任务数据
			taskData := executor.TaskData{
				TaskID:          i, // 使用索引作为taskid
				ID:              id,
				FromAddressPart: fromAddrPart,
				AddressMask:     processRet,
				PrefixCount:     prefixCount,
				SuffixCount:     suffixCount,
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

// GenerateMatcher 根据公钥生成前 N 后 M 的匹配串（格式：前 N 位... 后 M 位）
// 处理逻辑：
// 1. 移除公钥可能包含的 "0x" 前缀
// 2. 提取处理后字符串的前 N 位和后 M 位
// 3. 拼接为 "前 N 位... 后 M 位" 格式
func GenerateMatcher(publicKey string, prefixCount, suffixCount int) (string, error) {
	// 步骤 1：去除可能的 0x 前缀（不区分大小写）
	processedKey := strings.TrimPrefix(strings.ToLower(publicKey), "0x")

	// 验证处理后的公钥长度是否足够
	minLength := prefixCount + suffixCount // 前 N 位 + 后 M 位 = 至少 N+M 位
	if len(processedKey) < minLength {
		return "", errors.New(fmt.Sprintf(" 公钥长度不足，处理后长度为 % d，至少需要 % d 位 ", len(processedKey), minLength))
	}

	// 步骤 2：提取前 N 位和后 M 位
	prefix := processedKey[:prefixCount]
	suffix := processedKey[len(processedKey)-suffixCount:]

	// 步骤 3：拼接成 "前 N... 后 M" 格式
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
	config          *config.Config
	dbClient        *database.MySQLClient
	executor        *executor.Executor
	lastIDFile      string
	lastProcessTime time.Time
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
		config:          cfg,
		dbClient:        dbClient,
		executor:        exec,
		lastIDFile:      "last_id.txt",
		lastProcessTime: time.Now(),
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

func (app *Application) getNewDataCount(lastID int64) (int, error) {
	cond := "id > " + fmt.Sprintf("%d", lastID)
	if app.config.App.QueryCondition != "" {
		cond = fmt.Sprintf("(%s) AND %s", app.config.App.QueryCondition, cond)
	}

	// 使用条件查询
	return app.dbClient.CountDataWithCondition(
		app.config.App.TableName,
		cond,
	)
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
		return app.executor.ExecuteBatchWithTaskData(taskDataList,
			app.config.App.MaxConcurrency,
			app.config.App.MaxTasks)
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
				"input_file":   fmt.Sprintf("input-%d.txt", td.TaskID),
				"output_file":  fmt.Sprintf("output-%d.txt", td.TaskID),
				"prefix_count": td.PrefixCount,
				"suffix_count": td.SuffixCount,
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

			// 根据publicKey获取前N后M的匹配串
			fromPart, _ := GenerateMatcher(publicKey, taskData.PrefixCount, taskData.SuffixCount)
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

// 获取循环间隔配置
func (app *Application) getLoopInterval() int {
	if app.config.App.LoopInterval > 0 {
		return app.config.App.LoopInterval
	}
	return 60 // 默认60秒
}

// 计算检查间隔（主间隔的1/10，最小1秒）
func (app *Application) calculateCheckInterval(interval int) time.Duration {
	checkInterval := time.Duration(interval) * time.Second / 10
	if checkInterval < time.Second {
		return time.Second
	}
	return checkInterval
}

// 加载上次处理的ID
func (app *Application) loadLastID() (int64, error) {
	return LoadLastID(app.lastIDFile)
}

// 保存最新处理的ID
func (app *Application) saveLastID(id int64) error {
	return SaveLastID(id, app.lastIDFile)
}

// 更新上次处理时间
func (app *Application) updateLastProcessTime() {
	app.lastProcessTime = time.Now()
}

// 检查是否达到触发条件
func (app *Application) checkTriggerConditions(lastID int64, interval int) (bool, string) {
	// 检查时间触发条件
	timeTriggered, timeReason := app.checkTimeCondition(interval)
	if timeTriggered {
		return true, timeReason
	}

	// 检查数据量触发条件（如果配置了阈值）
	if app.config.App.TriggerThreshold > 0 {
		dataTriggered, dataReason := app.checkDataCondition(lastID)
		if dataTriggered {
			return true, dataReason
		}
		return false, dataReason
	}

	return false, "未达到时间间隔和数据阈值"
}

// 检查时间触发条件
func (app *Application) checkTimeCondition(interval int) (bool, string) {
	elapsed := time.Since(app.lastProcessTime)
	required := time.Duration(interval) * time.Second

	if elapsed >= required {
		return true, fmt.Sprintf("时间间隔触发（已过%v，要求%v）", elapsed, required)
	}
	return false, fmt.Sprintf("时间未到（已过%v，还需%v）", elapsed, required-elapsed)
}

// 检查数据量触发条件
func (app *Application) checkDataCondition(lastID int64) (bool, string) {
	count, err := app.getNewDataCount(lastID)
	if err != nil {
		return false, fmt.Sprintf("数据检查失败: %v", err)
	}

	if count >= app.config.App.TriggerThreshold {
		return true, fmt.Sprintf("数据量触发（新增%d条，阈值%d条）", count, app.config.App.TriggerThreshold)
	}
	return false, fmt.Sprintf("数据量不足（新增%d条，阈值%d条）", count, app.config.App.TriggerThreshold)
}

// 处理一个批次的数据
func (app *Application) processDataBatch(lastID int64) (int64, error) {
	// 1. 查询数据
	data, err := app.queryData(lastID)
	if err != nil {
		return lastID, fmt.Errorf("查询数据失败: %w", err)
	}

	if len(data) == 0 {
		log.Println("没有新数据需要处理")
		return lastID, nil
	}

	log.Printf("开始处理批次数据，共%d条记录", len(data))
	startTime := time.Now()

	// 2. 解析数据为任务列表
	taskDataList, err := parseData(data)
	if err != nil {
		return lastID, fmt.Errorf("解析数据失败: %w", err)
	}

	if len(taskDataList) == 0 {
		log.Println("没有有效任务需要执行")
		return app.getMaxID(data, lastID), nil
	}

	log.Printf("解析出%d个有效任务", len(taskDataList))

	// 3. 处理输入数据
	if err := app.processInputData(taskDataList); err != nil {
		return lastID, fmt.Errorf("处理输入数据失败: %w", err)
	}

	// 4. 执行第三方程序
	results, err := app.executeCommands(taskDataList)
	if err != nil {
		return lastID, fmt.Errorf("执行命令失败: %w", err)
	}

	// 5. 处理执行结果
	if err := app.processResults(taskDataList); err != nil {
		log.Printf("处理结果时警告: %v", err) // 非致命错误，继续执行
	}

	// 6. 清理临时文件
	app.cleanupFiles(taskDataList)

	// 7. 打印统计信息
	executionTime := time.Since(startTime)
	app.printStatistics(data, results, executionTime)

	// 8. 返回本次处理的最大ID
	return app.getMaxID(data, lastID), nil
}

// 获取数据中的最大ID
func (app *Application) getMaxID(data []map[string]interface{}, currentMax int64) int64 {
	maxID := currentMax
	for _, row := range data {
		if idVal, ok := row["id"]; ok {
			id := parseID(idVal)
			if id > maxID {
				maxID = id
			}
		}
	}
	return maxID
}

// runLoop 循环执行主流程，支持定时和last_id
func (app *Application) runLoop() error {
	// 初始化配置参数
	interval := app.getLoopInterval()
	checkInterval := app.calculateCheckInterval(interval)

	log.Printf("启动监控循环 - 定时间隔: %ds, 检查间隔: %v, 触发阈值: %d条",
		interval, checkInterval, app.config.App.TriggerThreshold)

	for {
		// 1. 加载上次处理的ID
		lastID, err := app.loadLastID()
		if err != nil {
			log.Printf("警告: 读取last_id失败: %v，将从0开始", err)
			lastID = 0
		}

		// 2. 检查是否达到触发条件
		triggered, reason := app.checkTriggerConditions(lastID, interval)
		if !triggered {
			log.Printf("未触发处理 - %s，%v后再次检查", reason, checkInterval)
			//app.updateLastProcessTime() //需要刷新lastProcessTime
			time.Sleep(checkInterval)
			continue
		}

		log.Printf("触发处理流程 - %s", reason)
		app.updateLastProcessTime()
		// 3. 执行数据处理流程
		maxID, err := app.processDataBatch(lastID)
		if err != nil {
			log.Printf("处理批次失败: %v", err)
		} else if maxID > lastID {
			// 4. 仅当有新数据处理时才更新状态
			if err := app.saveLastID(maxID); err != nil {
				log.Printf("保存last_id失败: %v", err)
			}
			log.Printf("批次处理完成，最新ID: %d，等待下次触发", maxID)
		}

		time.Sleep(checkInterval)
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
