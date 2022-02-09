package got

import "testing"

func Test_quoteText(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"standard text", "abc", "`abc`"},
		{"one quote text", `ab"c`, "`ab\"c`"},
		{"two quote text", `a"b"c`, "`a\"b\"c`"},
		{"backtick", "a`c", "`a` + \"`\" + `c`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteText(tt.val); got != tt.want {
				t.Errorf("quoteText() = %v, want %v", got, tt.want)
			}
		})
	}
}
