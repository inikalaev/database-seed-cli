// Package buildplugins compiles a directory of user-provided Go mechanism files
// into an augmented `seed` binary and delegates execution to it.
//
// Model: the CLI is a Go toolkit. When --generators ./dir is passed, the main
// process writes a throwaway module that imports both this CLI's cmd/seed/cli
// package and every .go file in ./dir (assumed to register mechanisms via
// `seedapi.Register` in an init()), then builds and execs it with the original
// argv (minus the --generators flag, which is consumed here).
//
// The resulting binary is cached in ${XDG_CACHE_HOME:-~/.cache}/seed-cli/<hash>.
// The hash covers: sha256 of every .go file in the generators dir + the
// resolved seed-cli source revision. Cache hits re-exec immediately.
//
// Requirements:
//   - Go toolchain installed (go in PATH).
//   - `SEED_CLI_SRC` env var pointing at this repo's checkout, OR the user's
//     generators dir contains a go.mod that already imports the published
//     seed-cli module. The former is the dev workflow; the latter is the
//     released-binary workflow (v2).
package buildplugins

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const modulePath = "github.com/ivannikolaev/seed-cli/cli"

// RunWithGenerators builds (or reuses) an augmented seed binary for the given
// generators dir and re-executes the current process through it. Returns only
// if the build step fails; on success it exec's and does not return.
func RunWithGenerators(genDir string, argv []string) error {
	absDir, err := filepath.Abs(genDir)
	if err != nil {
		return fmt.Errorf("generators path: %w", err)
	}
	srcDir := os.Getenv("SEED_CLI_SRC")
	if srcDir == "" {
		return fmt.Errorf("SEED_CLI_SRC env var is not set — point it at this repo's ./cli directory to use --generators in MVP")
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("SEED_CLI_SRC: %w", err)
	}

	hash, err := hashGeneratorDir(absDir, absSrc)
	if err != nil {
		return err
	}
	cacheBase, err := cacheDir()
	if err != nil {
		return err
	}
	buildDir := filepath.Join(cacheBase, hash)
	binPath := filepath.Join(buildDir, "seed")

	if _, err := os.Stat(binPath); err != nil {
		// Start from a clean buildDir so a leftover go.sum from a prior failed
		// `go mod tidy` doesn't poison this attempt.
		if err := os.RemoveAll(buildDir); err != nil {
			return fmt.Errorf("clean stale build dir: %w", err)
		}
		if err := materialize(buildDir, absDir, absSrc); err != nil {
			return err
		}
		if err := goBuild(buildDir, binPath); err != nil {
			// Leave no partially-built dir behind — next run will retry fresh.
			_ = os.RemoveAll(buildDir)
			return err
		}
	}

	child := exec.Command(binPath, argv...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = append(os.Environ(), "SEED_CLI_AUGMENTED=1")
	if err := child.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}

func cacheDir() (string, error) {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "seed-cli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "seed-cli"), nil
}

func hashGeneratorDir(dir, src string) (string, error) {
	h := sha256.New()
	fmt.Fprintln(h, "seed-cli-src:", src)
	entries, err := listGoFiles(dir)
	if err != nil {
		return "", err
	}
	for _, f := range entries {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		fmt.Fprintln(h, "file:", filepath.Base(f))
		h.Write(data)
	}
	// Hash every .go file under SEED_CLI_SRC so edits to the CLI invalidate
	// the augmented-binary cache. Without this, dev iteration on the core
	// re-executes stale cached binaries.
	if err := hashTreeGoFiles(h, src); err != nil {
		return "", err
	}
	// Dependency changes (new imports, version bumps) don't touch .go files
	// but must still invalidate the cache.
	for _, f := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		fmt.Fprintln(h, "dep:", f)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func hashTreeGoFiles(h io.Writer, root string) error {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, f)
		fmt.Fprintln(h, "core:", rel)
		if _, err := h.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func listGoFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read generators dir %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no .go files found in %s", dir)
	}
	return out, nil
}

func materialize(buildDir, genDir, srcDir string) error {
	if err := os.MkdirAll(filepath.Join(buildDir, "userplugins"), 0o755); err != nil {
		return err
	}
	files, err := listGoFiles(genDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := rewriteWithPackage(f, filepath.Join(buildDir, "userplugins", filepath.Base(f))); err != nil {
			return err
		}
	}
	goVersion := readGoDirective(filepath.Join(srcDir, "go.mod"))
	goMod := fmt.Sprintf(`module seed-augmented

go %s

require %s v0.0.0

replace %s => %s
`, goVersion, modulePath, modulePath, srcDir)
	if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		return err
	}
	mainGo := fmt.Sprintf(`package main

import (
	_ "seed-augmented/userplugins"

	"fmt"
	"os"

	"%s/cmd/seed/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
`, modulePath)
	return os.WriteFile(filepath.Join(buildDir, "main.go"), []byte(mainGo), 0o644)
}

// readGoDirective extracts the `go X.Y[.Z]` directive from a go.mod file.
// Falls back to "1.22" if the file is missing or malformed — the augmented
// build will still succeed on any reasonably recent toolchain.
func readGoDirective(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "1.22"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "go "); ok {
			v := strings.TrimSpace(rest)
			if v != "" {
				return v
			}
		}
	}
	return "1.22"
}

func rewriteWithPackage(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	text := string(in)
	// Find the package clause at the start of a line so we don't match
	// "package" appearing inside a doc comment or string literal above it.
	idx := -1
	if strings.HasPrefix(text, "package ") {
		idx = 0
	} else if i := strings.Index(text, "\npackage "); i >= 0 {
		idx = i + 1
	}
	if idx < 0 {
		return fmt.Errorf("file %s has no package clause", src)
	}
	end := strings.IndexByte(text[idx:], '\n')
	if end < 0 {
		return fmt.Errorf("file %s has no newline after package clause", src)
	}
	rewritten := text[:idx] + "package userplugins" + text[idx+end:]
	return os.WriteFile(dst, []byte(rewritten), 0o644)
}

func goBuild(buildDir, binPath string) error {
	// Resolve deps first.
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = buildDir
	tidy.Stdout = io.Discard
	tidy.Stderr = os.Stderr
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = buildDir
	build.Stdout = io.Discard
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("go build augmented seed: %w", err)
	}
	return nil
}
