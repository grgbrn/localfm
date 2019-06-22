package util

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
