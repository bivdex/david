// 接收从Go程序传递过来的参数
const args = process.argv.slice(2);
//console.log('len:'+args.length);
//console.log(args)
if (args.length < 1) {
    console.error("需要1个参数: private key");
    process.exit(1);
}

const privateKey = args[0];

// 处理数据
let result = 'fail';
const { ethers } = require("ethers");

try {
    //const privateKey = "0x17ef2d2cea89448d9bfcd90a29cfe0ad7c9f5dfae6dfc2b0d56d2c6a88745c8d";  // 你的私钥
    const wallet = new ethers.Wallet(privateKey);
    result = 'succ';
} catch (error) {
    //console.error("无效的私钥:", error.message);
}

// 将结果输出，供Go程序捕获
console.log(result);

// 退出程序
process.exit(0);
