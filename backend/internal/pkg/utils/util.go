package utils

/*
此文件存放与当前项目有关的工具函数，

若可抽象为通用工具函数，应放置于 "codeup.aliyun.com/qimao/leo/lib"包下
*/

import (
	"crypto/md5"
	"fmt"
	"hash/fnv"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"codeup.aliyun.com/qimao/leo/lib/pkg/json"
	"github.com/spf13/cast"
)

// Set 元素去重
func Set[T comparable](arr []T) []T {
	m := make(map[T]struct{}, len(arr))
	list := make([]T, 0, len(arr))
	for _, v := range arr {
		if _, ok := m[v]; !ok {
			list = append(list, v)
			m[v] = struct{}{}
		}
	}
	return list
}

// Include 判断是否在切片中存在
func Include[T comparable](arr []T, check T) bool {
	for _, v := range arr {
		if v == check {
			return true
		}
	}
	return false
}

// IsUnion 判断是否存在交集
func IsUnion[T comparable](arr1 []T, arr2 []T) bool {
	m := make(map[T]struct{}, len(arr1))
	for _, v := range arr1 {
		m[v] = struct{}{}
	}
	for _, v := range arr2 {
		if _, ok := m[v]; ok {
			return true
		}
	}
	return false
}

// GetStructName 获取当前函数所依赖名称 业务用于获取领域名称
func GetStructName(myvar any) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}

// ByteToString 避免直接转换带了额外内存分配，使用断言转换string
func ByteToString(data []byte) string {
	return *(*string)(unsafe.Pointer(&data))
}

// ToString 序列化为json
func ToString(data any) string {
	b, _ := json.Marshal(data)
	return *(*string)(unsafe.Pointer(&b))
}

// IsNum 判断字符串是否是数字
// 传值 "3.14", name 会按照3.14搜索 ，不需要这样搜索则使用ParseFloat
func IsNum(s string) bool {
	_, err := strconv.ParseInt(s, 0, 10)
	return err == nil
}

// IsRepByLoop 判断切片中是否存在重复元素
func IsRepByLoop[T comparable](origin []T) bool {
	set := Set(origin)
	return len(origin) != len(set)
}

func GetBucketId(str string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(str))
	return int64(h.Sum32()%1000 + 1)
}

// IntSliceToString int64切片转任意格式字符串
func IntSliceToString(data []int64, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(data), " ", delim, -1), "[]")
}

func IntSliceToStringSlice(data []int64) []string {
	res := make([]string, 0, len(data))
	for _, v := range data {
		res = append(res, strconv.FormatInt(v, 10))
	}
	return res
}

// IntSliceToMap 转客户端特殊结构 ["123(ID)":"1"] Value 没有含义
func IntSliceToMap(s []int64) map[string]string {
	m := make(map[string]string, len(s))
	for _, v := range s {
		m[strconv.FormatInt(v, 10)] = "1"
	}
	return m
}

func IsStringIn(targetSlice []string, element string) bool {
	return ContainsString(targetSlice, element) != -1
}

func IsInt64In(targetSlice []int64, element int64) bool {
	if len(targetSlice) == 0 {
		return false
	}
	for _, v := range targetSlice {
		if v == element {
			return true
		}
	}
	return false
}

func ContainsString(array []string, val string) (index int) {
	index = -1
	for i := 0; i < len(array); i++ {
		if array[i] == val {
			index = i
			return
		}
	}
	return
}

// Ternary 三目运算的函数
func Ternary(a bool, b, c any) any {
	if a {
		return b
	}
	return c
}

func getRandNumber() string {
	numSlice := make([]string, 0)
	for i := 0; i < 16; i++ {
		num := rand.Int31n(10)
		numSlice = append(numSlice, strconv.Itoa(int(num)))
	}
	return strings.Join(numSlice, "")
}

func UniqueId() string {
	rand.Seed(time.Now().UnixNano())
	return Md5(time.Now().String() + strconv.Itoa(int(rand.Uint32())))
}

func Md5(content string) (md string) {
	h := md5.New()
	h.Write([]byte(content))
	md = fmt.Sprintf("%x", h.Sum(nil))
	return
}

// IsOnline 判断是否在 上线状态 1上线   0下线
func IsOnline(upTime, downTime int64) int64 {
	var isOnline int64 = 0
	now := time.Now().Unix()
	if (upTime <= now && downTime > now) || (upTime < now && downTime >= now) {
		isOnline = 1
	}
	return isOnline
}

// ZeroAsEmptyString 0转为空字符串
func ZeroAsEmptyString(i int64) string {
	if i == 0 {
		return ""
	}
	return cast.ToString(i)
}
