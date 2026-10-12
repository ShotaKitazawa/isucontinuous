package imports

import (
	"os"
	"testing"
)

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		in      string
		want    os.FileMode
		wantErr bool
	}{
		{in: "644", want: 0644},
		{in: "755\n", want: 0755},
		{in: "600", want: 0600},
		{in: "0", want: 0},
		{in: "abc", wantErr: true},
		{in: "888", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseFileMode(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseFileMode(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseFileMode(%q) = %o, want %o", tt.in, got, tt.want)
		}
	}
}
