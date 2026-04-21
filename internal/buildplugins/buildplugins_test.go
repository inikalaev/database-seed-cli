package buildplugins_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inikalaev/database-seed-cli/internal/buildplugins"
)

// TestBuildAndRunWithFactories compiles an augmented binary with the custom
// my_rating factory from testdata/buildplugins/factories, runs generate, and
// verifies the output contains the expected table and rating values in the range 1–5.
//
// Skipped when:
//   - `go` is not in PATH
//   - SEED_CLI_SRC is not set (auto-computed from the test file location)
func TestBuildAndRunWithFactories(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}

	// Compute SEED_CLI_SRC relative to this test file:
	// cli/internal/buildplugins/ → ../../ = cli/
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	srcDir := filepath.Join(wd, "../..")
	t.Setenv("SEED_CLI_SRC", srcDir)

	genDir := filepath.Join(wd, "../../testdata/buildplugins/factories")
	binPath, err := buildplugins.Build(genDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	cfgPath := filepath.Join(wd, "../../testdata/buildplugins/seed.yaml")
	outFile := filepath.Join(t.TempDir(), "seed.sql")

	cmd := exec.Command(binPath, "generate", "-c", cfgPath, "-o", outFile)
	cmd.Env = append(os.Environ(), "SEED_CLI_AUGMENTED=1", "SEED_CLI_SRC="+srcDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate failed: %v\n%s", err, out)
	}

	sql, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	sqlStr := string(sql)

	if !strings.Contains(sqlStr, `"public"."reviews"`) {
		t.Errorf("missing reviews INSERT; got:\n%s", sqlStr)
	}
	// my_rating generates values 1..5; verify no out-of-range values appear.
	for _, bad := range []string{", 0)", ", 6)", ", 7)", ", 8)", ", 9)"} {
		if strings.Contains(sqlStr, bad) {
			t.Errorf("rating out of [1,5] range (%q found); got:\n%s", bad, sqlStr)
		}
	}
	if !strings.Contains(sqlStr, "BEGIN;") || !strings.Contains(sqlStr, "COMMIT;") {
		t.Errorf("missing transaction frame; got:\n%s", sqlStr)
	}
}
