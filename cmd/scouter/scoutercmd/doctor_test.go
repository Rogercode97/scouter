package scoutercmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRunDoctorChecks_ReportsPassAndFail(t *testing.T) {
	ctx := context.Background()

	fOut, _ := os.CreateTemp("", "doctor-out")
	defer os.Remove(fOut.Name())

	checks := []doctorCheck{
		{name: "passing", run: func(ctx context.Context) (string, error) { return "all good", nil }},
		{name: "failing", run: func(ctx context.Context) (string, error) {
			return "", errors.New("broken (remedy: fix it)")
		}},
	}

	if !runDoctorChecks(ctx, fOut, checks) {
		t.Fatal("expected failed=true when a probe fails")
	}
	fOut.Close()

	data, _ := os.ReadFile(fOut.Name())
	out := string(data)
	if !strings.Contains(out, "ok   passing — all good") {
		t.Errorf("missing pass line.\nGot:\n%s", out)
	}
	if !strings.Contains(out, "FAIL failing — broken (remedy: fix it)") {
		t.Errorf("missing fail line with remedy.\nGot:\n%s", out)
	}
}

func TestRunDoctorChecks_AllPass(t *testing.T) {
	ctx := context.Background()

	fOut, _ := os.CreateTemp("", "doctor-out")
	defer os.Remove(fOut.Name())

	checks := []doctorCheck{
		{name: "a", run: func(ctx context.Context) (string, error) { return "yes", nil }},
	}

	if runDoctorChecks(ctx, fOut, checks) {
		t.Error("expected failed=false when every probe passes")
	}
}

func TestDoctorCheck_AstGrepResolves(t *testing.T) {
	ctx := context.Background()

	var astg *doctorCheck
	for i := range defaultDoctorChecks("unused") {
		if defaultDoctorChecks("unused")[i].name == "ast-grep" {
			c := defaultDoctorChecks("unused")[i]
			astg = &c
			break
		}
	}
	if astg == nil {
		t.Fatal("default checks must include ast-grep")
	}
	ok, err := astg.run(ctx)
	if err != nil {
		t.Fatalf("ast-grep should resolve in dev and CI: %v", err)
	}
	if !strings.Contains(ok, "sg") && !strings.Contains(ok, "ast-grep") {
		t.Errorf("ok line should name the binary, got %q", ok)
	}
}
