#include <algorithm>
#include <stdexcept>
#include <iostream>
#include <fstream>
#include <sstream>
#include <cstdlib>
#include <cstdio>
#include <vector>
#include <map>
#include <set>
#include <signal.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#endif

#if defined(__APPLE__) || defined(__MACOSX)
#include <OpenCL/cl.h>
#include <OpenCL/cl_ext.h> // Included to get topology to get an actual unique identifier per device
#else
#include <CL/cl.h>
#include <CL/cl_ext.h> // Included to get topology to get an actual unique identifier per device
#endif

#define CL_DEVICE_PCI_BUS_ID_NV  0x4008
#define CL_DEVICE_PCI_SLOT_ID_NV 0x4009

#include "Dispatcher.hpp"
#include "ArgParser.hpp"
#include "Mode.hpp"
#include "help.hpp"

// 定义全局Dispatcher指针
Dispatcher* g_dispatcher = nullptr;

// 信号处理函数
void signalHandler(int signum) {
    if (g_dispatcher) {
        std::cout << "\nReceived interrupt signal. Saving results..." << std::endl;
        g_dispatcher->saveResults();
        exit(signum);
    }
}

std::string readFile(const char * const szFilename)
{
    
	std::ifstream in(szFilename, std::ios::in | std::ios::binary);
    if (!in.is_open()) {
        std::cout << "Cannot open the file: " << szFilename << std::endl;
        throw std::runtime_error(std::string("Cannot open the file: ") + szFilename);
    }

    std::string contents;
    in.seekg(0, std::ios::end);
    contents.resize(in.tellg());
    in.seekg(0, std::ios::beg);
    in.read(&contents[0], contents.size());
    in.close();

    if (contents.empty()) {
        std::cout << "Warning: File " << szFilename << " is empty" << std::endl;
    } else {
        std::cout << "Successfully read file: " << szFilename << " (size: " << contents.size() << " bytes)" << std::endl;
    }

    return contents;
}

std::vector<cl_device_id> getAllDevices(cl_device_type deviceType = CL_DEVICE_TYPE_GPU)
{
	std::vector<cl_device_id> vDevices;

	cl_uint platformIdCount = 0;
	clGetPlatformIDs (0, NULL, &platformIdCount);

	std::vector<cl_platform_id> platformIds (platformIdCount);
	clGetPlatformIDs (platformIdCount, platformIds.data (), NULL);

	for( auto it = platformIds.cbegin(); it != platformIds.cend(); ++it ) {
		cl_uint countDevice;
		clGetDeviceIDs(*it, deviceType, 0, NULL, &countDevice);

		std::vector<cl_device_id> deviceIds(countDevice);
		clGetDeviceIDs(*it, deviceType, countDevice, deviceIds.data(), &countDevice);

		std::copy( deviceIds.begin(), deviceIds.end(), std::back_inserter(vDevices) );
	}

	return vDevices;
}

template <typename T, typename U, typename V, typename W>
T clGetWrapper(U function, V param, W param2) {
	T t;
	function(param, param2, sizeof(t), &t, NULL);
	return t;
}

template <typename U, typename V, typename W>
std::string clGetWrapperString(U function, V param, W param2) {
	size_t len;
	function(param, param2, 0, NULL, &len);
	char * const szString = new char[len];
	function(param, param2, len, szString, NULL);
	std::string r(szString);
	delete[] szString;
	return r;
}

template <typename T, typename U, typename V, typename W>
std::vector<T> clGetWrapperVector(U function, V param, W param2) {
	size_t len;
	function(param, param2, 0, NULL, &len);
	len /= sizeof(T);
	std::vector<T> v;
	if (len > 0) {
		T * pArray = new T[len];
		function(param, param2, len * sizeof(T), pArray, NULL);
		for (size_t i = 0; i < len; ++i) {
			v.push_back(pArray[i]);
		}
		delete[] pArray;
	}
	return v;
}

std::vector<std::string> getBinaries(cl_program & clProgram) {
	std::vector<std::string> vReturn;
	auto vSizes = clGetWrapperVector<size_t>(clGetProgramInfo, clProgram, CL_PROGRAM_BINARY_SIZES);
	if (!vSizes.empty()) {
		unsigned char * * pBuffers = new unsigned char *[vSizes.size()];
		for (size_t i = 0; i < vSizes.size(); ++i) {
			pBuffers[i] = new unsigned char[vSizes[i]];
		}

		clGetProgramInfo(clProgram, CL_PROGRAM_BINARIES, vSizes.size() * sizeof(unsigned char *), pBuffers, NULL);
		for (size_t i = 0; i < vSizes.size(); ++i) {
			std::string strData(reinterpret_cast<char *>(pBuffers[i]), vSizes[i]);
			vReturn.push_back(strData);
			delete[] pBuffers[i];
		}

		delete[] pBuffers;
	}

	return vReturn;
}

unsigned int getUniqueDeviceIdentifier(const cl_device_id & deviceId) {
/*
	#if defined(CL_DEVICE_TOPOLOGY_AMD)
	auto topology = clGetWrapper<cl_device_topology_amd>(clGetDeviceInfo, deviceId, CL_DEVICE_TOPOLOGY_AMD);
	if (topology.raw.type == CL_DEVICE_TOPOLOGY_TYPE_PCIE_AMD) {
		return (topology.pcie.bus << 16) + (topology.pcie.device << 8) + topology.pcie.function;
	}
#endif
*/
	cl_int bus_id = clGetWrapper<cl_int>(clGetDeviceInfo, deviceId, CL_DEVICE_PCI_BUS_ID_NV);
	cl_int slot_id = clGetWrapper<cl_int>(clGetDeviceInfo, deviceId, CL_DEVICE_PCI_SLOT_ID_NV);
	return (bus_id << 16) + slot_id;
}

template <typename T> bool printResult(const T & t, const cl_int & err) {
	std::cout << ((t == NULL) ? toString(err) : "OK") << std::endl;
	return t == NULL;
}

bool printResult(const cl_int err) {
	std::cout << ((err != CL_SUCCESS) ? toString(err) : "OK") << std::endl;
	return err != CL_SUCCESS;
}

std::string getDeviceCacheFilename(cl_device_id & d, const size_t & inverseSize) {
	const auto uniqueId = getUniqueDeviceIdentifier(d);
	return "cache-opencl." + toString(inverseSize) + "." + toString(uniqueId);
}

cl_program buildProgram(cl_context & clContext, const std::vector<cl_device_id> & vDevices) {
    // 获取当前工作目录
    char currentPath[1024] = {0};
    #ifdef _WIN32
    GetCurrentDirectoryA(sizeof(currentPath), currentPath);
    #else
    getcwd(currentPath, sizeof(currentPath));
    #endif

    // 读取内核文件
    const std::string strKernel = readFile("keccak.cl");
    const std::string strVanity = readFile("profanity.cl");
    const char * szKernel[] = { strKernel.c_str(), strVanity.c_str() };
    const size_t szKernelLengths[] = { strKernel.length(), strVanity.length() };
    
    cl_int errorCode;
    cl_program clProgram = clCreateProgramWithSource(clContext, 2, szKernel, szKernelLengths, &errorCode);
    if (errorCode != CL_SUCCESS) {
        std::cout << "Failed to create OpenCL program: " << toString(errorCode) << std::endl;
        return NULL;
    }

    // 添加所有必要的编译选项
    std::stringstream ss;
    ss << "-D PROFANITY_INVERSE_SIZE=255";
    ss << " -D PROFANITY_MAX_SCORE=40";
    ss << " -I .";  // 添加当前目录到包含路径
    #ifdef _WIN32
    ss << " -D _WIN32";
    #endif
    
    // 对于NVIDIA GPU，添加一些优化选项
    for (const auto& device : vDevices) {
        char vendor[256];
        clGetDeviceInfo(device, CL_DEVICE_VENDOR, sizeof(vendor), vendor, NULL);
        std::string vendorStr(vendor);
        if (vendorStr.find("NVIDIA") != std::string::npos) {
            ss << " -cl-nv-verbose";  // 启用NVIDIA详细日志
            break;
        }
    }
    
    std::string buildOptions = ss.str();
    errorCode = clBuildProgram(clProgram, vDevices.size(), vDevices.data(), buildOptions.c_str(), NULL, NULL);
    
    if (errorCode != CL_SUCCESS) {
        // 获取并显示编译错误信息
        for (size_t i = 0; i < vDevices.size(); ++i) {
            size_t logSize;
            clGetProgramBuildInfo(clProgram, vDevices[i], CL_PROGRAM_BUILD_LOG, 0, NULL, &logSize);
            
            if (logSize > 1) {
                std::vector<char> log(logSize);
                clGetProgramBuildInfo(clProgram, vDevices[i], CL_PROGRAM_BUILD_LOG, logSize, log.data(), NULL);
                
                // 获取设备名称
                char deviceName[256];
                clGetDeviceInfo(vDevices[i], CL_DEVICE_NAME, sizeof(deviceName), deviceName, NULL);
                
                std::cout << "Device " << deviceName << " compile log:" << std::endl;
                std::cout << std::string(80, '-') << std::endl;
                std::cout << log.data() << std::endl;
                std::cout << std::string(80, '-') << std::endl;
            }
        }
        
        std::cout << "Failed to compile OpenCL program: " << toString(errorCode) << std::endl;
        clReleaseProgram(clProgram);
        return NULL;
    }
    return clProgram;
}

int main(int argc, char * * argv) {
	try {
		// 检查speed.txt是否存在
		std::ifstream speedFile("speed.txt");
		bool needBenchmark = !speedFile.is_open();
		double savedSpeed = 0.0;
		
		if (!needBenchmark) {
			std::string speedStr;
			std::getline(speedFile, speedStr);
			speedFile.close();
			
			try {
				savedSpeed = std::stod(speedStr);
				if (savedSpeed <= 0) {
					needBenchmark = true;
				} else {
					// 将MH/s转换回原始值
					savedSpeed = savedSpeed * 1000000; // 转换回H/s
				}
			} catch (...) {
				needBenchmark = true;
			}
		}

		if (needBenchmark) {
			std::cout << "First, you need to benchmark the speed." << std::endl;
			
			// 使用benchmark模式进行测速
			std::vector<cl_device_id> vFoundDevices = getAllDevices();
			if (vFoundDevices.empty()) {
				std::cout << "No available GPU devices found" << std::endl;
				return 1;
			}

			// 创建OpenCL上下文
			cl_int errorCode;
			auto clContext = clCreateContext(NULL, vFoundDevices.size(), vFoundDevices.data(), NULL, NULL, &errorCode);
			if (errorCode != CL_SUCCESS) {
				std::cout << "Failed to create OpenCL context" << std::endl;
				return 1;
			}
			auto clProgram = buildProgram(clContext, vFoundDevices);
			if (!clProgram) {
				clReleaseContext(clContext);
				return 1;
			}

			// 创建Dispatcher进行测速
			Mode benchmarkMode = Mode::benchmark();
			Dispatcher d(clContext, clProgram, benchmarkMode, 65536, 255, 16384, 0);
			
			// 添加所有设备
			for (size_t i = 0; i < vFoundDevices.size(); ++i) {
				d.addDevice(vFoundDevices[i], 64, i);
			}

			// 设置测速标志
			d.setBenchmarkMode(true, 4); // 3秒测速
			
			// 运行测速
			d.run();

			// 获取最大速度并保存到speed.txt
			double maxSpeed = d.getMaxSpeed();
			// 将速度转换为MH/s并取整
			int roundedSpeed = static_cast<int>((maxSpeed / 1000000) + 0.5); // 转换为MH/s并四舍五入
			
			std::ofstream outFile("speed.txt");
			if (!outFile.is_open()) {
				std::cout << "Unable to create speed.txt file" << std::endl;
				clReleaseProgram(clProgram);
				clReleaseContext(clContext);
				return 1;
			}
			outFile << roundedSpeed;
			outFile.close();

			std::cout << "\nGood, you can continue." << std::endl;
			// 清理OpenCL资源
			clReleaseProgram(clProgram);
			clReleaseContext(clContext);
			
			// 直接退出程序
			return 0;
		}

		// 继续原有的参数解析和主程序逻辑
		ArgParser argp(argc, argv);
		bool bHelp = false;
		bool bModeBenchmark = false;
		bool bModeZeros = false;
		bool bModeLetters = false;
		bool bModeNumbers = false;
		std::string strModeLeading;
		std::string strModeMatching;
		bool bModeLeadingRange = false;
		bool bModeRange = false;
		bool bModeMirror = false;
		bool bModeDoubles = false;

		// 新增9个模式的变量
		int nLeadingSeqLen = 0;     // 开头连续性长度
		int nAnySeqLen = 0;         // 任意位置连续性长度
		int nEndingSeqLen = 0;      // 结尾连续性长度
		std::string strLeadingSpec; // 开头指定字符
		std::string strAnySpec;     // 任意位置指定字符
		std::string strEndingSpec;  // 结尾指定字符
		int nLeadingSameLen = 0;    // 开头连续相同长度
		int nAnySameLen = 0;        // 任意位置连续相同长度
		int nEndingSameLen = 0;     // 结尾连续相同长度

		int rangeMin = 0;
		int rangeMax = 0;
		std::vector<size_t> vDeviceSkipIndex;
		size_t worksizeLocal = 64;
		size_t worksizeMax = 0; // Will be automatically determined later if not overriden by user
		bool bNoCache = false;
		size_t inverseSize = 255;
		size_t inverseMultiple = 16384;
		bool bMineContract = false;

		// 现有的参数
		argp.addSwitch('h', "help", bHelp);
		argp.addSwitch('0', "benchmark", bModeBenchmark);
		argp.addSwitch('1', "zeros", bModeZeros);
		argp.addSwitch('2', "letters", bModeLetters);
		argp.addSwitch('3', "numbers", bModeNumbers);
		argp.addSwitch('4', "leading", strModeLeading);
		argp.addSwitch('5', "matching", strModeMatching);
		argp.addSwitch('6', "leading-range", bModeLeadingRange);
		argp.addSwitch('7', "range", bModeRange);
		argp.addSwitch('8', "mirror", bModeMirror);
		argp.addSwitch('9', "leading-doubles", bModeDoubles);

		// 新增9个模式的参数
		argp.addSwitch('A', "leading-seq", nLeadingSeqLen);      // 开头连续性
		argp.addSwitch('B', "any-seq", nAnySeqLen);             // 任意位置连续性
		argp.addSwitch('C', "ending-seq", nEndingSeqLen);       // 结尾连续性
		argp.addSwitch('D', "leading-spec", strLeadingSpec);    // 开头指定字符
		argp.addSwitch('E', "any-spec", strAnySpec);           // 任意位置指定字符
		argp.addSwitch('F', "ending-spec", strEndingSpec);     // 结尾指定字符
		argp.addSwitch('G', "leading-same", nLeadingSameLen);   // 开头连续相同
		argp.addSwitch('H', "any-same", nAnySameLen);          // 任意位置连续相同
		argp.addSwitch('J', "ending-same", nEndingSameLen);    // 结尾连续相同

		argp.addSwitch('m', "min", rangeMin);
		argp.addSwitch('M', "max", rangeMax);
		argp.addMultiSwitch('s', "skip", vDeviceSkipIndex);
		argp.addSwitch('w', "work", worksizeLocal);
		argp.addSwitch('W', "work-max", worksizeMax);
		argp.addSwitch('n', "no-cache", bNoCache);
		argp.addSwitch('i', "inverse-size", inverseSize);
		argp.addSwitch('I', "inverse-multiple", inverseMultiple);
		argp.addSwitch('c', "contract", bMineContract);
		
		// 添加输出文件参数
		std::string outputFile;
		argp.addSwitch('o', "output", outputFile);

		if (!argp.parse()) {
			std::cout << "error: bad arguments, try again :<" << std::endl;
			return 1;
		}

		// 如果指定了-o参数但没有提供文件名，则退出
		if (!outputFile.empty() && outputFile == "true") {
			std::cout << "Error: -o parameter requires an output filename" << std::endl;
			return 1;
		}

		if (bHelp) {
			std::cout << g_strHelp << std::endl;
			return 0;
		}

		Mode mode = Mode::benchmark();
		if (bModeBenchmark) {
			mode = Mode::benchmark();
		} else if (bModeZeros) {
			mode = Mode::zeros();
		} else if (bModeLetters) {
			mode = Mode::letters();
		} else if (bModeNumbers) {
			mode = Mode::numbers();
		} else if (!strModeLeading.empty()) {
			mode = Mode::leading(strModeLeading.front());
		} else if (!strModeMatching.empty()) {
			mode = Mode::matching(strModeMatching);
		} else if (bModeLeadingRange) {
			mode = Mode::leadingRange(rangeMin, rangeMax);
		} else if (bModeRange) {
			mode = Mode::range(rangeMin, rangeMax);
		} else if(bModeMirror) {
			mode = Mode::mirror();
		} else if (bModeDoubles) {
			mode = Mode::doubles();
		}
		// 新增9个模式的判断
		else if (nLeadingSeqLen > 0) {
			mode = Mode::leadingSequential(nLeadingSeqLen);
		} else if (nAnySeqLen > 0) {
			mode = Mode::anySequential(nAnySeqLen);
		} else if (nEndingSeqLen > 0) {
			mode = Mode::endingSequential(nEndingSeqLen);
		} else if (!strLeadingSpec.empty()) {
			mode = Mode::leadingSpecific(strLeadingSpec);
		} else if (!strAnySpec.empty()) {
			mode = Mode::anySpecific(strAnySpec);
		} else if (!strEndingSpec.empty()) {
			mode = Mode::endingSpecific(strEndingSpec);
		} else if (nLeadingSameLen > 0) {
			mode = Mode::leadingSame(nLeadingSameLen);
		} else if (nAnySameLen > 0) {
			mode = Mode::anySame(nAnySameLen);
		} else if (nEndingSameLen > 0) {
			mode = Mode::endingSame(nEndingSameLen);
		} else {
			std::cout << g_strHelp << std::endl;
			return 0;
		}
		std::cout << "Mode: " << mode.name << std::endl;

		if (bMineContract) {
			mode.target = CONTRACT;
		} else {
			mode.target = ADDRESS;
		}
		std::cout << "Target: " << mode.transformName() << std:: endl;

		std::vector<cl_device_id> vFoundDevices = getAllDevices();
		std::vector<cl_device_id> vDevices;
		std::map<cl_device_id, size_t> mDeviceIndex;

		std::vector<std::string> vDeviceBinary;
		std::vector<size_t> vDeviceBinarySize;
		cl_int errorCode;
		bool bUsedCache = false;

		std::cout << "Devices:" << std::endl;
		for (size_t i = 0; i < vFoundDevices.size(); ++i) {
			// Ignore devices in skip index
			if (std::find(vDeviceSkipIndex.begin(), vDeviceSkipIndex.end(), i) != vDeviceSkipIndex.end()) {
				continue;
			}

			cl_device_id & deviceId = vFoundDevices[i];

			const auto strName = clGetWrapperString(clGetDeviceInfo, deviceId, CL_DEVICE_NAME);
			const auto computeUnits = clGetWrapper<cl_uint>(clGetDeviceInfo, deviceId, CL_DEVICE_MAX_COMPUTE_UNITS);
			const auto globalMemSize = clGetWrapper<cl_ulong>(clGetDeviceInfo, deviceId, CL_DEVICE_GLOBAL_MEM_SIZE);
			bool precompiled = false;

			// Check if there's a prebuilt binary for this device and load it
			if(!bNoCache) {
				std::ifstream fileIn(getDeviceCacheFilename(deviceId, inverseSize), std::ios::binary);
				if (fileIn.is_open()) {
					vDeviceBinary.push_back(std::string((std::istreambuf_iterator<char>(fileIn)), std::istreambuf_iterator<char>()));
					vDeviceBinarySize.push_back(vDeviceBinary.back().size());
					precompiled = true;
				}
			}

			std::cout << "  GPU" << i << ": " << strName << ", " << globalMemSize << " bytes available, " << computeUnits << " compute units (precompiled = " << (precompiled ? "yes" : "no") << ")" << std::endl;
			vDevices.push_back(vFoundDevices[i]);
			mDeviceIndex[vFoundDevices[i]] = i;
		}

		if (vDevices.empty()) {
			return 1;
		}

		std::cout << std::endl;
		std::cout << "Initializing OpenCL..." << std::endl;
		std::cout << "  Creating context..." << std::flush;
		auto clContext = clCreateContext( NULL, vDevices.size(), vDevices.data(), NULL, NULL, &errorCode);
		if (printResult(clContext, errorCode)) {
			return 1;
		}

		cl_program clProgram;
		if (vDeviceBinary.size() == vDevices.size()) {
			// Create program from binaries
			bUsedCache = true;

			std::cout << "  Loading kernel from binary..." << std::flush;
			const unsigned char * * pKernels = new const unsigned char *[vDevices.size()];
			for (size_t i = 0; i < vDeviceBinary.size(); ++i) {
				pKernels[i] = reinterpret_cast<const unsigned char *>(vDeviceBinary[i].data());
			}

			cl_int * pStatus = new cl_int[vDevices.size()];

			clProgram = clCreateProgramWithBinary(clContext, vDevices.size(), vDevices.data(), vDeviceBinarySize.data(), pKernels, pStatus, &errorCode);
			if(printResult(clProgram, errorCode)) {
				return 1;
			}
		} else {
			// Create a program from the kernel source
			std::cout << "  Compiling kernel..." << std::flush;
			const std::string strKeccak = readFile("keccak.cl");
			const std::string strVanity = readFile("profanity.cl");
			const char * szKernels[] = { strKeccak.c_str(), strVanity.c_str() };

			clProgram = clCreateProgramWithSource(clContext, sizeof(szKernels) / sizeof(char *), szKernels, NULL, &errorCode);
			if (printResult(clProgram, errorCode)) {
				return 1;
			}
		}

		// Build the program
		std::cout << "  Building program..." << std::flush;
		const std::string strBuildOptions = "-D PROFANITY_INVERSE_SIZE=" + toString(inverseSize) + " -D PROFANITY_MAX_SCORE=" + toString(PROFANITY_MAX_SCORE);
		if (printResult(clBuildProgram(clProgram, vDevices.size(), vDevices.data(), strBuildOptions.c_str(), NULL, NULL))) {
#ifdef PROFANITY_DEBUG
			std::cout << std::endl;
			std::cout << "build log:" << std::endl;

			size_t sizeLog;
			clGetProgramBuildInfo(clProgram, vDevices[0], CL_PROGRAM_BUILD_LOG, 0, NULL, &sizeLog);
			char * const szLog = new char[sizeLog];
			clGetProgramBuildInfo(clProgram, vDevices[0], CL_PROGRAM_BUILD_LOG, sizeLog, szLog, NULL);

			std::cout << szLog << std::endl;
			delete[] szLog;
#endif
			return 1;
		}

		// Save binary to improve future start times
		if( !bUsedCache && !bNoCache ) {
			std::cout << "  Saving program..." << std::flush;
			auto binaries = getBinaries(clProgram);
			for (size_t i = 0; i < binaries.size(); ++i) {
				std::ofstream fileOut(getDeviceCacheFilename(vDevices[i], inverseSize), std::ios::binary);
				fileOut.write(binaries[i].data(), binaries[i].size());
			}
			std::cout << "OK" << std::endl;
		}

		std::cout << std::endl;

		Dispatcher d(clContext, clProgram, mode, worksizeMax == 0 ? inverseSize * inverseMultiple : worksizeMax, inverseSize, inverseMultiple, 0);
		
		// 设置全局指针和信号处理
		g_dispatcher = &d;
		signal(SIGINT, signalHandler);  // Ctrl+C
		signal(SIGTERM, signalHandler); // 终止信号

		for (auto & i : vDevices) {
			d.addDevice(i, worksizeLocal, mDeviceIndex[i]);
		}

		// 如果有保存的速度值，设置重置间隔
		if (savedSpeed > 0) {
			// 将分钟转换为毫秒 (分钟 * 60秒/分 * 1000毫秒/秒)
			double minutes = (1.0 / (savedSpeed / 1000000)) * 1000; // 先将速度转换为MH/s再计算
			std::chrono::milliseconds resetInterval(static_cast<long long>(minutes * 60 * 1000)); // 转为毫秒
			d.setResetInterval(resetInterval);
			std::cout.precision(2);
			std::cout << std::fixed;
			std::cout << "Reset interval set to " << minutes << " minutes based on speed " << static_cast<int>(savedSpeed / 1000000) << " MH/s" << std::endl;
		}

		// 如果指定了输出文件，设置输出模式
		if (!outputFile.empty()) {
			d.setOutputMode(true, outputFile);
		}

		d.run();
		clReleaseContext(clContext);
		return 0;
	} catch (std::runtime_error & e) {
		std::cout << "std::runtime_error - " << e.what() << std::endl;
	} catch (...) {
		std::cout << "unknown exception occured" << std::endl;
	}

	return 1;
}

