package main

import (
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// 1. 私钥字符串（带0x前缀）
	privateKeyHex := "0x2a8c2393858bf6ec0b4cff4ff4b414eea99b17197b433423ebf09e54d446f93b" // 测试用私钥

	// 2. 解析私钥（去掉0x前缀）
	privateKey, err := crypto.HexToECDSA(privateKeyHex[2:])
	if err != nil {
		log.Fatalf("解析私钥失败: %v", err)
	}

	// 3. 获取公钥
	publicKey := privateKey.PublicKey
	publicKeyBytes := crypto.FromECDSAPub(&publicKey)
	publicKeyHex := fmt.Sprintf("0x%x", publicKeyBytes)

	// 4. 获取地址
	address := crypto.PubkeyToAddress(publicKey)
	addressHex := address.Hex()

	// 5. 输出结果
	fmt.Println("私钥解析结果：")
	fmt.Printf("公钥 (未压缩): %s\n", publicKeyHex)
	fmt.Printf("钱包地址: %s\n", addressHex)
}
