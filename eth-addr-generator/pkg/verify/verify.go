package verify

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"log"
	"os/exec"
	"strings"
)

func IsValidPrivateKey(targetKey, targetPubKey string) bool {
	// 2. 解析私钥（去掉0x前缀）
	privateKey, err := crypto.HexToECDSA(targetKey[2:])
	if err != nil {
		log.Fatalf("解析私钥失败: %v", err)
		return false
	}

	// 3. 获取公钥
	publicKey := privateKey.PublicKey
	// 4. 获取地址
	address := crypto.PubkeyToAddress(publicKey)
	addressHex := strings.ToLower(address.Hex())

	return targetPubKey == addressHex
}

func IsValidPrivateKeyByNodeJs(targetKey string) bool {
	// 定义要传递给Node.js脚本的参数
	privateKey := targetKey //"0x17ef2d2cea89448d9bfcd90a29cfe0ad7c9f5dfae6dfc2b0d56d2c6a88745c8d"

	// 执行node命令，调用脚本并传递参数
	cmd := exec.Command("node", "verify.js", privateKey)

	// 捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	err := cmd.Run()
	if err != nil {
		log.Fatalf("执行命令出错: %v\n stderr: %s", err, stderr.String())
		return false
	}

	// 比对结果
	result := stdout.String()
	//fmt.Println(result)
	if result == "succ\n" {
		fmt.Println("验证成功！")
		return true
	} else {
		fmt.Println("验证失败！")
		return false
	}

}
