package capture

import "testing"

func TestIndependentCaptureRouting(t *testing.T) {
	for _, tc := range []struct {
		mode string
		uid  int
		want string
	}{
		{"", -1, ""}, {"", 42, ""},
		{"AcceptSelected", 42, ""}, {"AcceptSelected", 99, "DIRECT"},
		{"DenySelected", 42, "DIRECT"}, {"DenySelected", 99, ""},
		{"AcceptSelected", -1, "REJECT"}, {"DenySelected", -1, "REJECT"},
	} {
		r := Routing{Mode: tc.mode, UIDs: []int{42}}
		if got := r.Proxy(tc.uid); got != tc.want {
			t.Fatalf("mode=%s uid=%d got=%s want=%s", tc.mode, tc.uid, got, tc.want)
		}
	}
	m := New(t.TempDir())
	m.Command("routing", `{"mode":"AcceptSelected","uids":[42]}`)
	old := m.Routing()
	m.Command("routing", `{"mode":"DenySelected","uids":[99]}`)
	if old.Proxy(42) != "" || old.Proxy(99) != "DIRECT" {
		t.Fatal("live tunnel routing changed")
	}
	copy := m.Routing()
	copy.UIDs[0] = 42
	if m.Routing().Proxy(99) != "DIRECT" {
		t.Fatal("snapshot aliases manager")
	}
	m.Stop()
	if old.Proxy(99) != "DIRECT" {
		t.Fatal("stopping capture changed direct routing")
	}
}
