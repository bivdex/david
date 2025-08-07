# 前N位后M位模式使用示例

## 概述
本系统现在支持任意前N位后M位的模式匹配，可以通过配置文件灵活调整。

## 配置方法

### 1. 前3位后4位模式（默认）
```yaml
app:
  prefix_count: 3
  suffix_count: 4
```
支持的格式：`abc...defg`

### 2. 前2位后3位模式
```yaml
app:
  prefix_count: 2
  suffix_count: 3
```
支持的格式：`ab...def`

### 3. 前5位后6位模式
```yaml
app:
  prefix_count: 5
  suffix_count: 6
```
支持的格式：`abcde...123456`

### 4. 前1位后2位模式
```yaml
app:
  prefix_count: 1
  suffix_count: 2
```
支持的格式：`a...bc`

## 输入输出示例

### 前3后4模式
- 输入：`abc...defg`
- 输出：`0xabcxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdefg`

### 前2后3模式
- 输入：`ab...def`
- 输出：`0xabxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdef`

### 前5后6模式
- 输入：`abcde...123456`
- 输出：`0xabcdexxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx123456`

## 验证规则

1. **前缀验证**：前N位必须是十六进制字符（0-9, a-f, A-F）
2. **中间分隔符**：必须是三个点号 `...`
3. **后缀验证**：后M位必须是十六进制字符（0-9, a-f, A-F）
4. **长度验证**：总长度必须等于 N + 3 + M

## 错误示例

### 前3后4模式下的错误输入
- `ab...defg` - 前缀不足3位
- `abc...def` - 后缀不足4位
- `abc..defg` - 中间分隔符错误
- `abc...defgh` - 后缀超过4位
- `abcd...defg` - 前缀超过3位

## 注意事项

1. **配置一致性**：确保配置文件中的 `prefix_count` 和 `suffix_count` 与数据库中的 `from_address_part` 字段格式一致
2. **第三方程序参数**：系统会自动将配置的前N位后M位参数传递给第三方程序
3. **向后兼容**：默认配置保持前3后4模式，现有系统无需修改即可继续使用
4. **性能考虑**：前N位后M位的值会影响匹配的精确度，建议根据实际需求合理设置
