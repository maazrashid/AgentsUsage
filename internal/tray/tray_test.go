package tray

import "testing"

func TestCompactTokens(t *testing.T) {
	tests := map[int64]string{0: "0", 999: "999", 1_250: "1.2K", 2_500_000: "2.5M", 3_100_000_000: "3.1B"}
	for input, expected := range tests {
		if got := compactTokens(input); got != expected {
			t.Fatalf("compactTokens(%d) = %q, want %q", input, got, expected)
		}
	}
}
