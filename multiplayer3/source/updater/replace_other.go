//go:build !windows
package main
import "os"
func replacePreparedFile(temp,target string)error{return os.Rename(temp,target)}
