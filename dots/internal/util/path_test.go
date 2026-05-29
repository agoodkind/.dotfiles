package util

import "testing"

func TestResolveConfigPath(t *testing.T) {
	type testCase struct {
		name     string
		value    string
		dotfiles string
		home     string
		xdgCache string
		want     string
	}
	cases := []testCase{
		{name: "empty value", value: "", dotfiles: "/repo", home: "/h", xdgCache: "", want: ""},
		{name: "home expands", value: "$HOME/.cache/x", dotfiles: "/repo", home: "/h", xdgCache: "", want: "/h/.cache/x"},
		{name: "dotdotfiles expands", value: "$DOTDOTFILES/zshrc/x.zsh", dotfiles: "/repo", home: "/h", xdgCache: "", want: "/repo/zshrc/x.zsh"},
		{name: "xdg cache unset falls back to home", value: "${XDG_CACHE_HOME}/dotfiles_dispatch.log", dotfiles: "/repo", home: "/h", xdgCache: "", want: "/h/.cache/dotfiles_dispatch.log"},
		{name: "xdg cache set is honored", value: "${XDG_CACHE_HOME}/dotfiles_dispatch.log", dotfiles: "/repo", home: "/h", xdgCache: "/custom/cache", want: "/custom/cache/dotfiles_dispatch.log"},
	}
	for _, testInput := range cases {
		t.Run(testInput.name, func(t *testing.T) {
			t.Setenv("HOME", testInput.home)
			t.Setenv("XDG_CACHE_HOME", testInput.xdgCache)
			got := ResolveConfigPath(testInput.value, testInput.dotfiles)
			if got != testInput.want {
				t.Errorf("ResolveConfigPath(%q, %q) = %q, want %q", testInput.value, testInput.dotfiles, got, testInput.want)
			}
		})
	}
}
