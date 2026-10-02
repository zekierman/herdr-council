package main

import "testing"

const defaultsSample = `
[keys]
# prefix = "ctrl+b"
# goto = "prefix+g"
# new_tab = "prefix+c"
# focus_pane_up = ["prefix+k", "prefix+up"]

[ui]
# accent = "cyan"
`

func TestBoundKeysAndPick(t *testing.T) {
	used := boundKeys(defaultsSample, true)
	for _, k := range []string{"prefix+g", "prefix+c", "prefix+k", "prefix+up"} {
		if !used[k] {
			t.Errorf("default %s not seen", k)
		}
	}
	if used["ctrl+b"] || used["cyan"] {
		t.Error("prefix and non-key values must not count as bindings")
	}

	user := `
[keys]
new_tab = "prefix+a"

[[keys.command]]
key = "prefix+shift+a"
type = "plugin_action"
command = "someone.else"
`
	for k := range boundKeys(user, false) {
		used[k] = true
	}
	got, err := pickKey("", used)
	if err != nil || got != "prefix+y" {
		t.Fatalf("pick: %q %v", got, err)
	}
	if _, err := pickKey("prefix+g", used); err == nil {
		t.Fatal("a taken key must be refused")
	}
	if got, _ := pickKey("prefix+z", used); got != "prefix+z" {
		t.Fatalf("explicit free key: %q", got)
	}
}

func TestExistingBinding(t *testing.T) {
	cfg := "[[keys.command]]\nkey = \"prefix+x\"\ncommand = \"other.thing\"\n\n[[keys.command]]\nkey = \"prefix+a\"\ntype = \"plugin_action\"\ncommand = \"herdr-council.open\"\n"
	if k := existingBinding(cfg); k != "prefix+a" {
		t.Fatalf("got %q", k)
	}
	if k := existingBinding("[[keys.command]]\nkey = \"prefix+x\"\ncommand = \"other.thing\"\n"); k != "" {
		t.Fatalf("got %q", k)
	}
}
