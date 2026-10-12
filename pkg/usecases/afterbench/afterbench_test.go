package afterbench

import "testing"

func TestMovedPath(t *testing.T) {
	m := Moved{From: "/p/abc", To: "/p/abc.1700000000"}
	for path, want := range map[string]string{
		"/p/abc/alp.txt":    "/p/abc.1700000000/alp.txt",
		"/p/abc/sub/pt.txt": "/p/abc.1700000000/sub/pt.txt",
		"/p/abc":            "/p/abc.1700000000",
		"/p/abcd/alp.txt":   "/p/abcd/alp.txt",
		"/elsewhere/pt.txt": "/elsewhere/pt.txt",
	} {
		if got := m.Path(path); got != want {
			t.Errorf("Path(%q) = %q, want %q", path, got, want)
		}
	}
	if got := (Moved{}).Path("/p/abc/alp.txt"); got != "/p/abc/alp.txt" {
		t.Errorf("a failed move must keep the path, got %q", got)
	}
}
