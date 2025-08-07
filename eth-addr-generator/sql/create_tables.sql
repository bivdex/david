
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;
-- 生成结果表
CREATE TABLE `t_order_address_record_result`  (
                                                  `id` bigint NOT NULL AUTO_INCREMENT,
                                                  `from_address_part` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '前三后四码',
                                                  `address` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '地址',
                                                  `create_time` timestamp NULL DEFAULT NULL COMMENT '创建时间',
                                                  `private_address` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '私钥',
                                                  `match_success_time` datetime NULL DEFAULT NULL COMMENT '匹配成功时间',
                                                  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 1933715030797217795 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '订单记录A表' ROW_FORMAT = DYNAMIC;

SET FOREIGN_KEY_CHECKS = 1;