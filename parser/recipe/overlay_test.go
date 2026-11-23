package recipe

import "testing"

func TestMergeOverlay(t *testing.T) {
	base := []Config{{Code: "a", Hosts: []string{"a.test"}}, {Code: "b", Hosts: []string{"b.test"}}}
	extra := []Config{{Code: "b", Hosts: []string{"b2.test"}}, {Code: "c", Hosts: []string{"c.test"}}}
	m := Merge(base, extra...)
	if len(m) != 3 {
		t.Fatalf("len=%d", len(m))
	}
	if m[1].Hosts[0] != "b2.test" {
		t.Fatalf("overlay b: %+v", m[1])
	}
	if m[2].Code != "c" {
		t.Fatalf("c missing")
	}
}

func TestParseConfigs(t *testing.T) {
	raw := []byte(`[{"code":"x","hosts":["x.test"],"path_regex":"^/p$"}]`)
	list, err := ParseConfigs(raw)
	if err != nil || len(list) != 1 || list[0].Code != "x" {
		t.Fatalf("%v %+v", err, list)
	}
}
