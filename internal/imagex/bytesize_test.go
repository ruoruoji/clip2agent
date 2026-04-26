package imagex

import "testing"

func TestParseByteSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"", 0},
		{"1", 1},
		{"1k", 1024},
		{"2m", 2 * 1024 * 1024},
		{"3g", 3 * 1024 * 1024 * 1024},
		{" 8M ", 8 * 1024 * 1024},
	}
	for _, c := range cases {
		got, err := ParseByteSize(c.in)
		if err != nil {
			t.Fatalf("ParseByteSize(%q) err=%v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParseByteSize(%q)=%d, want %d", c.in, got, c.want)
		}
	}
}
