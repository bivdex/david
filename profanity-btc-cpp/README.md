# 1.安装gcc
mingw下载链接：https://github.com/niXman/mingw-builds-binaries/releases 
选择x86_64-15.1.0-release-win32-seh-msvcrt-rt_v12-rev0.7z
或者直接下载下面这个链接：
https://github.com/niXman/mingw-builds-binaries/releases/download/15.1.0-rt_v12-rev0/x86_64-15.1.0-release-win32-seh-msvcrt-rt_v12-rev0.7z
## 解压后mingw包
找到bin目录路径 配置到环境变量中
## 测试g++命令
测试g++是否为有效命令
# 2.编译
```
./compile.bat
```

# 3.运行
拷贝整个bin目录到其他地方或机器 运行bin/profanity-btc.exe


# c++调用opencl并行计算原理
名词解释
主机程序：触发cl计算的c++程序
cl内核程序：clBuildProgram创建的在gpu上运行的程序

1、主机程序c++根据命令行参数构建不同的Mode 不同的命令其实对应kernal函数和入参数据
kernal函数实际就是profanity.cl实现的函数
2、clBuildProgram 创建cl内核程序
3、Dispatcher.run 提交给gpu设备运行 
4、GPU运行.cl的函数进行计算
5、监听计算完成事件 接受计算结果

