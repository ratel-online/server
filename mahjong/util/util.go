package util

import (
	"math/rand"
	"strconv"
	"time"
)

// GenRangeInt 创建一个长度为length的slice
// 值为连续的整形，第一个数为from
func GenRangeInt(length int, from int) []int {
	_range := make([]int, length, length)
	for i := 0; i < length; i++ {
		_range[i] = i + from
	}
	return _range
}

// IntInSlice 判断某个int值是否在切片中
func IntInSlice(finder int, slice []int) bool {
	exists := false
	for _, v := range slice {
		if v == finder {
			exists = true
			break
		}
	}
	return exists
}

// ShuffleSliceInt 打乱一个切片
func ShuffleSliceInt(src []int) []int {
	dest := make([]int, len(src))

	rand.Seed(time.Now().UTC().UnixNano())
	perm := rand.Perm(len(src))

	for i, v := range perm {
		dest[v] = src[i]
	}

	return dest
}

// IsSameSlice 判断两个slice是否相等
func IsSameSlice(slice1, slice2 []int) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for i := 0; i < len(slice1); i++ {
		if slice1[i] != slice2[i] {
			return false
		}
	}
	return true
}

// SliceDel 删除slice中的某些元素
func SliceDel(slice []int, values ...int) []int {
	if slice == nil || len(values) == 0 {
		return slice
	}
	for _, value := range values {
		slice = sliceDel(slice, value)
	}
	return slice
}

func sliceDel(slice []int, value int) []int {
	if slice == nil {
		return slice
	}
	for i, j := range slice {
		if j == value {
			return append(append([]int{}, slice[:i]...), slice[i+1:]...)
		}
	}
	return slice
}

// SliceCopy 拷贝一个切片
func SliceCopy(s []int) []int {
	var slice = make([]int, len(s))
	copy(slice, s)
	return slice
}

// SliceJoin 将一个slice拼接成一个字符串
func SliceJoin(s []int, joinString string) string {
	var str = ""
	if length := len(s); length > 0 {
		str = strconv.Itoa(s[0])
		for i := 1; i < length; i++ {
			str += joinString
			str += strconv.Itoa(s[i])
		}
	}
	return str
}

// InStringSlice 判断某个string值是否在切片中
func InStringSlice(finder string, slice []string) bool {
	exists := false
	for _, v := range slice {
		if v == finder {
			exists = true
			break
		}
	}
	return exists
}

// SliceDelString 删除slice中的某些元素
func SliceDelString(slice []string, values ...string) []string {
	if slice == nil || len(values) == 0 {
		return slice
	}
	for _, value := range values {
		slice = sliceDelString(slice, value)
	}
	return slice
}

func sliceDelString(slice []string, value string) []string {
	if slice == nil {
		return slice
	}
	for i, j := range slice {
		if j == value {
			// return append(slice[:i], slice[i+1:]...)
			return append(append([]string{}, slice[:i]...), slice[i+1:]...)
		}
	}
	return slice
}

// SliceMaxInt 取int类型的最大值
func SliceMaxInt(s []int) int {
	var max = 0
	for _, v := range s {
		if v > max {
			max = v
		}
	}
	return max
}

// SliceUniqueInt 去重
func SliceUniqueInt(s []int) []int {
	uniquedSlice := []int{}
	m := make(map[int]bool)
	for _, v := range s {
		if _, exists := m[v]; !exists {
			m[v] = true
			uniquedSlice = append(uniquedSlice, v)
		}
	}
	return uniquedSlice
}

// SliceToMap 将[]int 转化成map[int]count
func SliceToMap(slice []int) map[int]int {
	var m = map[int]int{}
	for _, j := range slice {
		var _, ok = m[j]
		if ok {
			m[j]++
		} else {
			m[j] = 1
		}
	}
	return m
}

// MapToSlice 将map[int]count 转成 []int
func MapToSlice(m map[int]int) []int {
	tiles := []int{}
	for tile, cnt := range m {
		for i := 0; i < cnt; i++ {
			tiles = append(tiles, tile)
		}
	}
	return tiles
}
