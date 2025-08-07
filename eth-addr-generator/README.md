# ETH Address Generator

一个用于读取MySQL数据库数据并调用第三方程序的Go应用程序。

## 功能特性

- 从MySQL指定表读取数据
- 将数据库记录作为参数调用第三方程序
- 支持条件查询和批量处理
- 支持并发执行
- 完整的执行结果统计
- 可配置的超时和重试机制

## 项目结构

```
eth-addr-generator/
├── cmd/
│   └── main.go              # 主程序入口
├── configs/
│   └── config.yaml          # 配置文件
├── pkg/
│   ├── config/
│   │   └── config.go        # 配置管理
│   ├── database/
│   │   └── mysql.go         # MySQL数据库操作
│   └── executor/
│       └── executor.go      # 第三方程序执行器
├── test_program.go          # 示例第三方程序
├── go.mod                   # Go模块文件
└── README.md               # 项目说明
```

## 安装和运行
### 1. 安装nodejs

### 2. 安装ethers库

```bash
npm install ethers
```

### 3. 安装依赖

```bash
go mod tidy
```

### 4. 编译程序

```bash
go build -o eth-acc-gen.exe cmd/main.go
```

### 5. 配置数据库

编辑 `configs/config.yaml` 文件，设置数据库连接信息：

```yaml
database:
  host: "127.0.0.1"
  port: 3307
  username: "root"
  password: "123456"
  database: "tron"
```

### 6. 配置应用参数

```yaml
app:
  table_name: "test_table"     # 要查询的表名
  query_limit: 100             # 查询数据限制
  max_concurrency: 5           # 最大并发数
  query_condition: ""          # 查询条件（可选）
  enable_batch_mode: false     # 是否启用批量模式

executor:
  program_path: "./test"       # 第三方程序路径
  timeout: 30                  # 执行超时时间（秒）
```

### 7. 运行程序

```bash
go run cmd/main.go
```
或者使用编译后的可执行程序：
```bash
./eth-acc-gen.exe
```

或者指定配置文件：

```bash
go run cmd/main.go configs/config.yaml
```

## 配置说明

### 数据库配置

- `host`: MySQL服务器地址
- `port`: MySQL端口
- `username`: 数据库用户名
- `password`: 数据库密码
- `database`: 数据库名称
- `max_idle_conns`: 最大空闲连接数
- `max_open_conns`: 最大打开连接数
- `max_lifetime`: 连接最大生命周期（秒）

### 执行器配置

- `program_path`: 第三方程序路径
- `args`: 固定参数列表
- `timeout`: 执行超时时间（秒）
- `working_dir`: 工作目录
- `env`: 环境变量

### 应用配置

- `table_name`: 要查询的表名
- `query_limit`: 查询数据限制
- `max_concurrency`: 最大并发数
- `query_condition`: 查询条件（SQL WHERE子句）
- `enable_batch_mode`: 是否启用批量并发模式

## 使用示例

### 1. 基本查询

查询 `users` 表的前100条记录：

```yaml
app:
  table_name: "users"
  query_limit: 100
  query_condition: ""
```

### 2. 条件查询

查询状态为活跃的用户：

```yaml
app:
  table_name: "users"
  query_limit: 50
  query_condition: "status = 'active'"
```

### 3. 并发执行

启用批量并发模式：

```yaml
app:
  enable_batch_mode: true
  max_concurrency: 10
```

### 4. 第三方程序参数

数据库表的列名会自动转换为命令行参数：

- 数据库列 `id` → `--id=值`
- 数据库列 `name` → `--name=值`
- 数据库列 `value` → `--value=值`

## 输出示例

```
开始执行 eth generator程序...
MySQL database connected successfully
Configuration loaded successfully
Found 5 records in table test_table
Starting sequential execution
Command executed successfully in 5.123s

=== 执行结果统计 ===
总记录数: 5
成功执行: 3
执行失败: 2
总执行时间: 25.456s
平均执行时间: 5.091s
成功率: 60.00%

程序执行完成！
```

## 注意事项

1. 确保MySQL数据库正在运行且可访问
2. 确保第三方程序存在且有执行权限
3. 数据库表的列名将作为命令行参数传递给第三方程序
4. 支持的数据类型：string, int, float64, bool
5. 程序会自动处理Windows和Unix系统的命令差异

## 错误处理

程序包含完整的错误处理机制：

- 数据库连接失败
- 查询执行失败
- 第三方程序执行超时
- 参数转换错误

所有错误都会记录到日志中，并在程序结束时显示统计信息。