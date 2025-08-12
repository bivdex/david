cmake_minimum_required(VERSION 3.20)
project(profanity-btc)
set(CMAKE_CXX_STANDARD 11)
# 全局设置可执行文件输出目录
set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ../bin)

# 添加你的源文件
file(GLOB SOURCES "src/*.cpp")
# 添加头文件
file(GLOB HEADERS
        "src/*.h"
        "src/*.hpp"
)

include_directories(opencl-sdk/include)
link_directories(opencl-sdk/lib)
link_libraries(OpenCL)

add_executable(${PROJECT_NAME} ${SOURCES} ${HEADERS})


