package util

import (
	"fmt"
	"os"
	"strconv"
)

// FileExists checks for presence of a filepath
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// Pow does simple integer exponentiation
// "a**b" in some other languages
func Pow(a, b int) int {
	p := 1
	for b > 0 {
		if b&1 != 0 {
			p *= a
		}
		b >>= 1
		a *= a
	}
	return p
}

// InterfaceSliceInt64 allows []int64 to be used as varargs
// (definitely an ugly corner of go). This is more complicated than
// a simple cast, as explained:
// https://github.com/golang/go/wiki/InterfaceSlice
func InterfaceSliceInt64(data []int64) []interface{} {
	var interfaceSlice = make([]interface{}, len(data))
	for i, d := range data {
		interfaceSlice[i] = d
	}
	return interfaceSlice
}

// GetEnvStr loads an environment variable as a string, returning a default value if unset
func GetEnvStr(name, missing string) string {
	val := os.Getenv(name)
	if val == "" {
		return missing
	}
	return val
}

// GetEnvInt loads an environment variable as a int, returning a default value if unset
func GetEnvInt(name string, missing int) int {
	val := os.Getenv(name)
	if val == "" {
		return missing
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		fmt.Printf("Error parsing %s as an int: %s", name, val)
		return missing
	}
	return n
}

// MustGetEnvStr loads a environment variable as a string or panics
func MustGetEnvStr(name string) string {
	val := os.Getenv(name)
	if val == "" {
		panic(fmt.Sprintf("Must set %s", name))
	}
	return val
}

// MustGetEnvInt loads an environment variable as an int or panics
func MustGetEnvInt(name string) int {
	val := os.Getenv(name)
	if val == "" {
		panic(fmt.Sprintf("Must set %s", name))
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		panic(fmt.Sprintf("Error parsing %s as an int: %s", name, val))
	}
	return n
}
