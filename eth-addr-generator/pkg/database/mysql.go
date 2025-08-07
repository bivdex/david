package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConfig 数据库配置
type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxLifetime  int    `mapstructure:"max_lifetime"`
}

// MySQLClient MySQL客户端
type MySQLClient struct {
	db *sql.DB
}

// NewMySQLClient 创建MySQL客户端
func NewMySQLClient(config MySQLConfig) (*MySQLClient, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Username, config.Password, config.Host, config.Port, config.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetConnMaxLifetime(time.Duration(config.MaxLifetime) * time.Second)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("MySQL database connected successfully")
	return &MySQLClient{db: db}, nil
}

// Close 关闭数据库连接
func (c *MySQLClient) Close() error {
	return c.db.Close()
}

// QueryData 查询指定表的数据
func (c *MySQLClient) QueryData(tableName string, limit int) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query table %s: %w", tableName, err)
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}

	// 为每行创建值的切片
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// 遍历结果集
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// 将当前行转换为map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			row[col] = val
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Successfully queried %d rows from table %s", len(results), tableName)
	return results, nil
}

// QueryDataWithCondition 根据条件查询数据
func (c *MySQLClient) QueryDataWithCondition(tableName, condition string, limit int) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT %d", tableName, condition, limit)

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query table %s with condition: %w", tableName, err)
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}

	// 为每行创建值的切片
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// 遍历结果集
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// 将当前行转换为map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			row[col] = val
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Successfully queried %d rows from table %s with condition", len(results), tableName)
	return results, nil
}

// UpdateData 根据条件更新数据
// params 为字段更新映射，如 {"name": "newName", "age": 20}
// condition 为WHERE条件，如 "id = 1 AND status = 0"
func (c *MySQLClient) UpdateData(tableName string, params map[string]interface{}, condition string) (int64, error) {
	// 参数校验
	if len(params) == 0 {
		return 0, fmt.Errorf("update parameters cannot be empty")
	}
	if tableName == "" {
		return 0, fmt.Errorf("table name cannot be empty")
	}
	if condition == "" {
		return 0, fmt.Errorf("update condition cannot be empty to prevent full table update")
	}

	// 构建更新字段部分
	var setClauses []string
	var args []interface{}
	argIndex := 1

	for col, val := range params {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", col))
		args = append(args, val)
		argIndex++
	}

	// 构建完整SQL
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		tableName,
		strings.Join(setClauses, ", "),
		condition)

	// 执行更新
	result, err := c.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute update: %w", err)
	}

	// 获取受影响的行数
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	log.Printf("Updated %d rows in table %s with condition: %s", rowsAffected, tableName, condition)
	return rowsAffected, nil
}

// UpdateByID 根据ID更新数据（简化常用场景）
// id 为记录的ID值
// idColumn 为ID列名，通常是"id"
// params 为字段更新映射
func (c *MySQLClient) UpdateByID(tableName, idColumn string, id interface{}, params map[string]interface{}) (int64, error) {
	if idColumn == "" {
		idColumn = "id" // 默认使用"id"作为ID列名
	}
	condition := fmt.Sprintf("%s = ?", idColumn)

	// 构建参数列表，将ID添加到参数末尾
	args := make([]interface{}, 0, len(params)+1)
	for _, val := range params {
		args = append(args, val)
	}
	args = append(args, id)

	// 构建更新字段部分
	var setClauses []string
	for col := range params {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", col))
	}

	// 构建完整SQL
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		tableName,
		strings.Join(setClauses, ", "),
		condition)

	// 执行更新
	result, err := c.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute %s update by ID: %w", query, err)
	}

	// 获取受影响的行数
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rowsAffected > 0 {
		log.Printf("Updated %d rows in table %s by %s = %s", rowsAffected, tableName, idColumn, id)
	}

	return rowsAffected, nil
}
