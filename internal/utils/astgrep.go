package utils

import (
	"fmt"
	"os/exec"
)

// astGrepCandidates is the search order for the ast-grep executable. Upstream
// ships it as `sg` and as `ast-grep`; distributions that package it may install
// only one of the two (Arch's ast-grep package installs only the long name), so
// hardcoding either one leaves half the machines with a silent "not found".
var astGrepCandidates = []string{"sg", "ast-grep"}

// AstGrepBinary returns the name of the installed ast-grep executable, or an
// error when no candidate is on PATH.
//
// It deliberately returns the bare name rather than the resolved absolute path:
// SafeCommand validates its binary against a closed allow-list of names, so a
// path would be rejected there and every structural refactor would fail on the
// machines that install only `ast-grep`.
func AstGrepBinary() (string, error) {
	for _, name := range astGrepCandidates {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("ast-grep is not installed: neither %q nor %q is in PATH",
		astGrepCandidates[0], astGrepCandidates[1])
}
