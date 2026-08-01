// Package local storage 的本地磁盘实现:按日期分目录保存文件,
// 删除密钥为 sha256(path+salt) 前 8 字节 hex。
package local
