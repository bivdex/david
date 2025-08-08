#ifndef HPP_DISPATCHER
#define HPP_DISPATCHER

#include <stdexcept>
#include <fstream>
#include <string>
#include <vector>
#include <mutex>
#include <atomic>
#include <iostream>

// 声明外部变量
extern class Dispatcher* g_dispatcher;

#if defined(__APPLE__) || defined(__MACOSX)
#include <OpenCL/cl.h>
#define clCreateCommandQueueWithProperties clCreateCommandQueue
#else
#include <CL/cl.h>
#endif

#include "SpeedSample.hpp"
#include "CLMemory.hpp"
#include "types.hpp"
#include "Mode.hpp"

#define PROFANITY_SPEEDSAMPLES 20
#define PROFANITY_MAX_SCORE 40

class Dispatcher {
	private:
		class OpenCLException : public std::runtime_error {
			public:
				OpenCLException(const std::string s, const cl_int res);

				static void throwIfError(const std::string s, const cl_int res);

				const cl_int m_res;
		};

		struct Device {
			static cl_command_queue createQueue(cl_context & clContext, cl_device_id & clDeviceId);
			static cl_kernel createKernel(cl_program & clProgram, const std::string s);
			cl_ulong4 createSeed();

			Device(Dispatcher & parent, cl_context & clContext, cl_program & clProgram, cl_device_id clDeviceId, const size_t worksizeLocal, const size_t size, const size_t index, const Mode & mode);
			~Device();

			Dispatcher & m_parent;
			const size_t m_index;

			cl_device_id m_clDeviceId;
			size_t m_worksizeLocal;
			cl_uchar m_clScoreMax;
			cl_command_queue m_clQueue;

			cl_kernel m_kernelInit;
			cl_kernel m_kernelInverse;
			cl_kernel m_kernelIterate;
			cl_kernel m_kernelTransform;
			cl_kernel m_kernelScore;

			CLMemory<point> m_memPrecomp;
			CLMemory<mp_number> m_memPointsDeltaX;
			CLMemory<mp_number> m_memInversedNegativeDoubleGy;
			CLMemory<mp_number> m_memPrevLambda;
			CLMemory<result> m_memResult;

			// Data parameters used in some modes
			CLMemory<cl_uchar> m_memData1;
			CLMemory<cl_uchar> m_memData2;

			// Seed and round information
			cl_ulong4 m_clSeed;
			cl_ulong m_round;

			// Speed sampling
			SpeedSample m_speed;

			// Initialization
			size_t m_sizeInitialized;
			cl_event m_eventFinished;
		};

	public:
		Dispatcher(cl_context & clContext, cl_program & clProgram, const Mode mode, const size_t worksizeMax, const size_t inverseSize, const size_t inverseMultiple, const cl_uchar clScoreQuit = 0);
		~Dispatcher();

		void addDevice(cl_device_id clDeviceId, const size_t worksizeLocal, const size_t index);
		void run();
		
		// 新增测速相关方法
		void setBenchmarkMode(bool enabled, int durationSeconds);
		double getMaxSpeed() const;
		void setResetInterval(std::chrono::milliseconds interval);

		// 新增输出模式相关方法
		void setOutputMode(bool enabled, const std::string& filename) {
			m_outputMode = enabled;
			m_outputFile = filename;
			m_foundCount = 0;
		}

		void addResult(const std::string& privateKey, const std::string& address, int score) {
			if (m_outputMode) {
				std::string result = privateKey + "-" + address + "-" + std::to_string(score);
				m_results.push_back(result);
				m_foundCount++;
			}
		}

		void saveResults() {
			if (m_outputMode && !m_results.empty()) {
				std::ofstream outFile(m_outputFile);
				if (outFile.is_open()) {
					for (const auto& result : m_results) {
						outFile << result << std::endl;
					}
					outFile.close();
					std::cout << "\nResults saved to " << m_outputFile << std::endl;
				}
			}
		}

	private:
		void init();
		void initBegin(Device & d);
		void initContinue(Device & d);

		void dispatch(Device & d);
		void enqueueKernel(cl_command_queue & clQueue, cl_kernel & clKernel, size_t worksizeGlobal, const size_t worksizeLocal, cl_event * pEvent);
		void enqueueKernelDevice(Device & d, cl_kernel & clKernel, size_t worksizeGlobal, cl_event * pEvent);

		void handleResult(Device & d);
		void randomizeSeed(Device & d);

		void onEvent(cl_event event, cl_int status, Device & d);

		void printSpeed();

	private:
		static void CL_CALLBACK staticCallback(cl_event event, cl_int event_command_exec_status, void * user_data);

		static std::string formatSpeed(double s);

	private: /* Instance variables */
		cl_context & m_clContext;
		cl_program & m_clProgram;
		const Mode m_mode;
		const size_t m_worksizeMax;
		const size_t m_inverseSize;
		const size_t m_size;
		cl_uchar m_clScoreMax;
		cl_uchar m_clScoreQuit;

		std::vector<Device *> m_vDevices;

		cl_event m_eventFinished;

		// Run information
		std::mutex m_mutex;
		std::chrono::time_point<std::chrono::steady_clock> timeStart;
		unsigned int m_countPrint;
		unsigned int m_countRunning;
		size_t m_sizeInitTotal;
		size_t m_sizeInitDone;
		bool m_quit;

		// Speed tracking and reset variables
		double m_maxSpeed;
		std::chrono::time_point<std::chrono::steady_clock> m_speedMeasureStart;
		bool m_speedMeasuring;
		std::chrono::milliseconds m_resetInterval;
		std::chrono::time_point<std::chrono::steady_clock> m_lastResetTime;

		// 新增测速相关变量
		bool m_benchmarkMode;
		int m_benchmarkDuration;
		std::chrono::time_point<std::chrono::steady_clock> m_benchmarkStart;

		// 新增输出模式相关变量
		bool m_outputMode;
		std::string m_outputFile;
		std::vector<std::string> m_results;
		std::atomic<size_t> m_foundCount;
};

#endif /* HPP_DISPATCHER */
