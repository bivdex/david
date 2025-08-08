#ifndef HPP_HELP
#define HPP_HELP

#include <string>

const std::string g_strHelp = R"(
使用方法: ./profanity [选项]

  连续性字符模式:
    --leading-seq <长度>    匹配开头有指定长度连续数字的地址
    --any-seq <长度>        匹配任意位置有指定长度连续数字的地址
    --ending-seq <长度>     匹配结尾有指定长度连续数字的地址

  指定字符模式:
    --leading-spec <hex>    匹配开头为指定十六进制字符串的地址
    --any-spec <hex>        匹配任意位置包含指定十六进制字符串的地址
    --ending-spec <hex>     匹配结尾为指定十六进制字符串的地址

  连续相同字符模式:
    --leading-same <长度>   匹配开头有指定长度相同字符的地址
    --any-same <长度>       匹配任意位置有指定长度相同字符的地址
    --ending-same <长度>    匹配结尾有指定长度相同字符的地址

  设备控制:
    -s, --skip <索引>      跳过指定索引的设备
    -n, --no-cache         不加载预编译的内核缓存
  
  输出:
    -o, --output <文件>    输出结果到指定文件

  性能调优:
    -w, --work <大小>      设置OpenCL本地工作组大小 [默认=64]
    -W, --work-max <大小>  设置OpenCL最大工作组大小 [默认=-i * -I]
    -i, --inverse-size     设置单个工作项计算的模逆数量 [默认=255]
    -I, --inverse-multiple 设置并行运行的工作项数量 [默认=16384]

  示例:
    # 基础模式示例
    ./profanity --leading f           # 寻找以'f'开头的地址
    ./profanity --matching dead       # 寻找包含'dead'的地址
    ./profanity --leading-range -m 0 -M 1  # 寻找开头字符在0-1范围内的地址
    ./profanity --contract --leading 0     # 寻找以'0'开头的合约地址

    # 连续性字符示例
    ./profanity --leading-seq 4    # 寻找开头有4个连续数字的地址
    ./profanity --any-seq 5        # 寻找任意位置有5个连续数字的地址
    ./profanity --ending-seq 3     # 寻找结尾有3个连续数字的地址

    # 指定字符示例
    ./profanity --leading-spec dead  # 寻找以"dead"开头的地址
    ./profanity --any-spec cafe      # 寻找包含"cafe"的地址
    ./profanity --ending-spec beef    # 寻找以"beef"结尾的地址

    # 连续相同字符示例
    ./profanity --leading-same 4    # 寻找开头有4个相同字符的地址
    ./profanity --any-same 6        # 寻找任意位置有6个相同字符的地址
    ./profanity --ending-same 5     # 寻找结尾有5个相同字符的地址)";

#endif /* HPP_HELP */