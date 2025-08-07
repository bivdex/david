package config

import (
	"fmt"
	"log"

	"github.com/bivdex/david/eth-addr-generator/pkg/database"
	"github.com/bivdex/david/eth-addr-generator/pkg/executor"
	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Database database.MySQLConfig    `mapstructure:"database"`
	Executor executor.ExecutorConfig `mapstructure:"executor"`
	App      AppConfig               `mapstructure:"app"`
}

// AppConfig 应用配置
type AppConfig struct {
	TableName        string `mapstructure:"table_name"`
	QueryLimit       int    `mapstructure:"query_limit"`
	MaxConcurrency   int    `mapstructure:"max_concurrency"`
	QueryCondition   string `mapstructure:"query_condition"`
	EnableBatchMode  bool   `mapstructure:"enable_batch_mode"`
	LoopInterval     int    `mapstructure:"loop_interval"` // 新增，单位：秒
	MaxTasks         int    `mapstructure:"max_tasks"`     // 新增，最大任务数
	TriggerThreshold int    `mapstructure:"trigger_threshold"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	log.Println("Configuration loaded successfully")
	return &config, nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 数据库默认配置
	viper.SetDefault("database.host", "127.0.0.1")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.database", "test")
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.max_open_conns", 100)
	viper.SetDefault("database.max_lifetime", 300)

	// 执行器默认配置
	viper.SetDefault("executor.program_path", "./test")
	viper.SetDefault("executor.timeout", 30)
	viper.SetDefault("executor.working_dir", ".")
	viper.SetDefault("executor.env", map[string]string{})

	// 应用默认配置
	viper.SetDefault("app.table_name", "test_table")
	viper.SetDefault("app.query_limit", 100)
	viper.SetDefault("app.max_concurrency", 5)
	viper.SetDefault("app.query_condition", "")
	viper.SetDefault("app.enable_batch_mode", false)
	viper.SetDefault("app.loop_interval", 60)     // 新增，设置默认值
	viper.SetDefault("app.max_tasks", 1000)       // 新增，设置默认值
	viper.SetDefault("app.trigger_threshold", 10) // 新增，设置默认值
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	// 验证数据库配置
	if config.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if config.Database.Port <= 0 {
		return fmt.Errorf("database port must be positive")
	}
	if config.Database.Username == "" {
		return fmt.Errorf("database username is required")
	}
	if config.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	// 验证执行器配置
	if config.Executor.ProgramPath == "" {
		return fmt.Errorf("executor program path is required")
	}

	// 验证应用配置
	if config.App.TableName == "" {
		return fmt.Errorf("table name is required")
	}
	if config.App.QueryLimit <= 0 {
		return fmt.Errorf("query limit must be positive")
	}
	if config.App.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be positive")
	}

	return nil
}

// GetDSN 获取数据库连接字符串
func (c *Config) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Database.Username, c.Database.Password, c.Database.Host, c.Database.Port, c.Database.Database)
}
