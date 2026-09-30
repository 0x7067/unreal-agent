package llm

import (
	"reflect"
	"testing"
)

func TestTokenUsageAddIncludesEveryCounter(t *testing.T) {
	var left, right, want TokenUsage
	l, r, w := reflect.ValueOf(&left).Elem(), reflect.ValueOf(&right).Elem(), reflect.ValueOf(&want).Elem()
	for index := range l.NumField() {
		l.Field(index).SetInt(int64(index + 1))
		r.Field(index).SetInt(int64(10 * (index + 1)))
		w.Field(index).SetInt(l.Field(index).Int() + r.Field(index).Int())
	}
	if got := left.Add(right); got != want {
		t.Fatalf("sum = %#v, want %#v", got, want)
	}
}
