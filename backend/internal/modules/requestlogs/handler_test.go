package requestlogs

import (
	"encoding/json"
	"testing"
)

func TestStatusFilterAcceptsEmptyAndNumericJSON(t *testing.T) {
	// 升级期间同时兼容旧前端的空字符串、数字字符串和新前端的数字值。
	tests := []struct {
		input string
		want  statusFilter
	}{
		{`{"status":""}`, 0},
		{`{"status":null}`, 0},
		{`{"status":"200"}`, 200},
		{`{"status":404}`, 404},
	}
	for _, test := range tests {
		var input listInput
		if err := json.Unmarshal([]byte(test.input), &input); err != nil {
			t.Fatalf("status %s rejected: %v", test.input, err)
		}
		if input.Status != test.want {
			t.Fatalf("status %s became %d, want %d", test.input, input.Status, test.want)
		}
	}
}

func TestStatusFilterRejectsInvalidJSON(t *testing.T) {
	// 非数字文本仍然失败，避免把错误筛选悄悄当成“不限状态”。
	var input listInput
	if err := json.Unmarshal([]byte(`{"status":"success"}`), &input); err == nil {
		t.Fatal("non-numeric status was accepted")
	}
}
