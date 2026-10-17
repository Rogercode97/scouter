package filter

import (
	"regexp"
	"testing"
)

// GoTestFailureRegex is the single entry point for identifying which file a test
// failure came from: internal/engine/diagnostic.go uses it in Diagnose,
// ExtractRCA and DiagnoseHUD, and semanticPurify uses it to enrich output. Its
// character class therefore decides whether scouter can diagnose a failure at
// all, and it admitted only [a-zA-Z0-9_/.], which no repository checked out
// under a hyphenated path can satisfy.
func TestGoTestFailureRegex_ParsesPathsTheToolchainProduces(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantPath string
		wantLine string
	}{
		{
			name:     "plain relative path",
			line:     "    main.go:7: boom",
			wantPath: "main.go",
			wantLine: "7",
		},
		{
			name:     "hyphenated absolute path",
			line:     "    /tmp/scouter-loc-ab12/w/internal/engine/test_bad_pkg/bad.go:4: some error",
			wantPath: "/tmp/scouter-loc-ab12/w/internal/engine/test_bad_pkg/bad.go",
			wantLine: "4",
		},
		{
			name:     "path with spaces",
			line:     "    /home/user/my projects/app/main.go:12: boom",
			wantPath: "/home/user/my projects/app/main.go",
			wantLine: "12",
		},
		{
			name:     "dotted directory names",
			line:     "    /a/b.c/d.go:3: x",
			wantPath: "/a/b.c/d.go",
			wantLine: "3",
		},
		{
			name:     "the first reference on the line wins",
			line:     "    /x/bad.go:4: undefined: foo.go:12:",
			wantPath: "/x/bad.go",
			wantLine: "4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := GoTestFailureRegex.FindStringSubmatch(tt.line)
			if m == nil {
				t.Fatalf("no match for %q", tt.line)
			}
			if m[1] != tt.wantPath {
				t.Errorf("path = %q, want %q", m[1], tt.wantPath)
			}
			if m[2] != tt.wantLine {
				t.Errorf("line = %q, want %q", m[2], tt.wantLine)
			}
		})
	}
}

func TestGoTestFailureRegex_NoMatchWithoutFileAndLine(t *testing.T) {
	for _, line := range []string{
		"    --- FAIL: TestFail (0.00s)",
		"    some error without a reference",
		"",
	} {
		if m := GoTestFailureRegex.FindStringSubmatch(line); m != nil {
			t.Errorf("unexpected match for %q: %v", line, m)
		}
	}
}

// The callers in internal/engine wrap this pattern with (?m) before scanning a
// whole log, so a match has to stay inside the line it started on.
func TestGoTestFailureRegex_MultilineLogStaysOnItsLine(t *testing.T) {
	re := regexp.MustCompile("(?m)" + GoTestFailureRegex.String())
	log := `
--- FAIL: TestFail (0.00s)
    /tmp/scouter-loc-ab12/w/internal/engine/test_bad_pkg/bad.go:4: some error
--- FAIL: TestOther (0.00s)
`
	m := re.FindStringSubmatch(log)
	if m == nil {
		t.Fatal("no match in the log")
	}
	if want := "/tmp/scouter-loc-ab12/w/internal/engine/test_bad_pkg/bad.go"; m[1] != want {
		t.Errorf("path = %q, want %q", m[1], want)
	}
	if m[2] != "4" {
		t.Errorf("line = %q, want \"4\"", m[2])
	}
}
