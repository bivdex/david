copy /Y src\*.cl bin\
copy /Y src\*.clh bin\
g++ ./src/Dispatcher.cpp ./src/Mode.cpp ./src/precomp.cpp ./src/profanity.cpp ./src/SpeedSample.cpp -std=c++11 -I"./opencl-sdk/include" -L"./opencl-sdk/lib" -lOpenCL -o ./bin/profanity-btc.exe
pause()