// mempool_test
package base

import (
	"fmt"
	"testing"
)

const (
	str = "hello gohelpers"
	sep = ","
)

func BenchmarkFmt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%s%s%s%s%s", str, sep, str, sep, str)
	}
}

func BenchmarkPlus(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = str + sep + str + sep + str
	}
}
