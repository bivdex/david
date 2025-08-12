# 比特币地址生成修改说明

## 概述

本修改将原来的以太坊地址生成器改造为比特币地址生成器，支持生成4种不同类型的比特币地址，并随机选择其中一种。

## 修改内容

### 1. 包含比特币地址生成头文件

在 `profanity.cl` 文件开头添加：
```opencl
#include "btc_addr.clh"
```

### 2. 修改主要迭代内核

`profanity_iterate` 内核现在生成比特币地址而不是以太坊地址：

- **随机选择地址类型**：每个线程随机选择 0-3 中的一种地址类型
- **生成压缩公钥**：从椭圆曲线点坐标生成压缩格式的公钥
- **支持四种地址类型**：
  - `case 0`: Legacy P2PKH (以 "1" 开头)
  - `case 1`: SegWit P2SH (以 "3" 开头)
  - `case 2`: Native SegWit P2WPKH (以 "bc1q" 开头)
  - `case 3`: Taproot P2TR (以 "bc1p" 开头，简化版本)

### 3. 新增内核函数

#### `profanity_generate_btc_addresses`
生成完整的比特币地址字符串，包括：
- 输入：公钥坐标和 lambda 值
- 输出：完整的比特币地址字符串
- 支持所有四种地址格式

#### `profanity_get_address_types`
获取每个线程选择的地址类型，用于调试和验证。

## 地址类型说明

### 1. Legacy P2PKH (Pay-to-Public-Key-Hash)
- 格式：以 "1" 开头的 Base58Check 编码地址
- 示例：`1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa`
- 用途：传统的比特币地址格式

### 2. SegWit P2SH (Pay-to-Script-Hash)
- 格式：以 "3" 开头的 Base58Check 编码地址
- 示例：`3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy`
- 用途：兼容的 SegWit 地址，支持更低的交易费用

### 3. Native SegWit P2WPKH (Pay-to-Witness-Public-Key-Hash)
- 格式：以 "bc1q" 开头的 Bech32 编码地址
- 示例：`bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4`
- 用途：原生 SegWit 地址，最低的交易费用

### 4. Taproot P2TR (Pay-to-Taproot)
- 格式：以 "bc1p" 开头的 Bech32m 编码地址
- 示例：`bc1p0xlxvlhemja6c4dqv22uapctqupfhlxm9h8z3k2e72q4k9hcz7vqzk5jj0`
- 用途：最新的比特币地址格式，支持 Schnorr 签名和脚本路径

## 使用方法

### 1. 编译
确保 `btc_addr.clh` 文件在正确的路径下，然后正常编译项目。

### 2. 调用内核
```cpp
// 调用主要的迭代内核
clEnqueueNDRangeKernel(queue, profanity_iterate_kernel, ...);

// 可选：生成完整地址字符串
clEnqueueNDRangeKernel(queue, profanity_generate_btc_addresses_kernel, ...);

// 可选：获取地址类型
clEnqueueNDRangeKernel(queue, profanity_get_address_types_kernel, ...);
```

### 3. 内存分配
```cpp
// 为比特币地址字符串分配内存（每条地址最多92字符）
size_t addressStride = 92;
char* btcAddresses = new char[globalSize * addressStride];

// 为地址类型分配内存
uchar* addressTypes = new uchar[globalSize];
```

## 性能考虑

1. **哈希计算**：比特币地址生成需要 SHA256 + RIPEMD160，比以太坊的 Keccak-256 稍慢
2. **编码复杂度**：Base58Check 和 Bech32 编码比简单的十六进制编码复杂
3. **内存使用**：需要额外的内存来存储完整的地址字符串

## 注意事项

1. **随机性**：地址类型选择基于线程ID，确保每个线程生成不同的地址类型
2. **兼容性**：保持与原有评分系统的兼容性，仍然使用20字节的 hash160 进行模式匹配
3. **错误处理**：当前实现简化了某些复杂的地址生成逻辑（如完整的 Taproot 实现）

## 扩展建议

1. **真正的随机性**：可以使用种子或时间戳来增加地址类型选择的随机性
2. **完整 Taproot 支持**：实现完整的 Schnorr 签名和脚本路径支持
3. **地址验证**：添加地址格式验证和校验和检查
4. **性能优化**：优化哈希计算和编码算法的性能

## 测试

建议使用以下测试向量验证地址生成：
- Legacy P2PKH: 已知的公钥和对应地址
- SegWit P2SH: 已知的脚本和对应地址
- Native SegWit: 已知的公钥和对应地址
- Taproot: 已知的公钥和对应地址
