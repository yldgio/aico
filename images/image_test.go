package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yldgio/aico/internal/runtime"
)

// TestMain lets this test binary double as a fake runtime CLI: when invoked
// with AICO_TEST_FAKE_BIN=1 it records its own argv to the file named by
// AICO_TEST_FAKE_BIN_RECORD and, for "image inspect"/"inspect" calls, prints a
// scripted stdout, then exits with a scripted code. This mirrors the pattern
// in internal/runtime/runtime_test.go so build argv and staleness branches
// can be asserted without a real docker/podman binary.
func TestMain(m *testing.M) {
	if os.Getenv("AICO_TEST_FAKE_BIN") == "1" {
		record := os.Getenv("AICO_TEST_FAKE_BIN_RECORD")
		_ = os.WriteFile(record, []byte(strings.Join(os.Args[1:], " ")), 0o644)
		if out := os.Getenv("AICO_TEST_FAKE_BIN_STDOUT"); out != "" {
			os.Stdout.WriteString(out)
		}
		// FAIL only simulates a missing image/label (ImageExists/ImageLabel
		// probes); a "build" invocation always "succeeds" so tests can
		// inspect the argv it was called with.
		isBuild := len(os.Args) > 1 && os.Args[1] == "build"
		code := 0
		if os.Getenv("AICO_TEST_FAKE_BIN_FAIL") == "1" && !isBuild {
			code = 1
		}
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fakeRuntime returns a Runtime whose Bin is this test binary re-invoked as a
// stub, plus a function to read back the argv it was last called with.
// stdout/fail control what the stub prints/returns for ImageExists/ImageLabel
// checks (i.e. "docker image inspect" / "docker inspect --format ...").
func fakeRuntime(t *testing.T, stdout string, fail bool) (*runtime.Runtime, func() []string) {
	t.Helper()
	record := filepath.Join(t.TempDir(), "argv")
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("AICO_TEST_FAKE_BIN", "1")
	t.Setenv("AICO_TEST_FAKE_BIN_RECORD", record)
	t.Setenv("AICO_TEST_FAKE_BIN_STDOUT", stdout)
	if fail {
		t.Setenv("AICO_TEST_FAKE_BIN_FAIL", "1")
	} else {
		t.Setenv("AICO_TEST_FAKE_BIN_FAIL", "0")
	}
	r := &runtime.Runtime{Bin: self}
	return r, func() []string {
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatalf("read record: %v", err)
		}
		return strings.Fields(string(data))
	}
}

func TestEnsureBuiltBuildsWithBaseTarget(t *testing.T) {
	// AICO_TEST_FAKE_BIN_FAIL=1 makes every stub call (including "image
	// inspect") fail, so EnsureBuilt treats the image as absent and proceeds
	// to build.
	r, argv := fakeRuntime(t, "", true)
	if err := EnsureBuilt(r); err != nil {
		t.Fatalf("EnsureBuilt: %v", err)
	}
	got := argv()
	if len(got) < 4 || got[0] != "build" {
		t.Fatalf("argv = %v, want it to start with build ...", got)
	}
	if !strings.Contains(strings.Join(got, " "), "--target base") {
		t.Errorf("argv = %v, want --target base", got)
	}
	if !strings.Contains(strings.Join(got, " "), "-t "+DefaultTag) {
		t.Errorf("argv = %v, want -t %s", got, DefaultTag)
	}
}

func TestEnsureDevenvBuiltBuildsWithDevenvTarget(t *testing.T) {
	r, argv := fakeRuntime(t, "", true)
	if err := EnsureDevenvBuilt(r); err != nil {
		t.Fatalf("EnsureDevenvBuilt: %v", err)
	}
	got := argv()
	if len(got) < 4 || got[0] != "build" {
		t.Fatalf("argv = %v, want it to start with build ...", got)
	}
	if !strings.Contains(strings.Join(got, " "), "--target devenv") {
		t.Errorf("argv = %v, want --target devenv", got)
	}
	if !strings.Contains(strings.Join(got, " "), "-t "+DevenvTag) {
		t.Errorf("argv = %v, want -t %s", got, DevenvTag)
	}
}

func TestEnsureDevenvBuiltSkipsWhenLabelMatchesHash(t *testing.T) {
	// The stub answers every runtime.Output call (both "image inspect" for
	// ImageExists and "inspect --format" for ImageLabel) with the current
	// content hash, so the image looks present and up to date and no build
	// should be attempted (the record file is only written on invocation, so
	// its final content must be the *last* call: the label lookup, not a
	// "build" argv).
	r, argv := fakeRuntime(t, contentHash()+"\n", false)
	if err := EnsureDevenvBuilt(r); err != nil {
		t.Fatalf("EnsureDevenvBuilt: %v", err)
	}
	got := argv()
	if len(got) > 0 && got[0] == "build" {
		t.Errorf("argv = %v, want no build when label matches hash", got)
	}
}

func TestEnsureBuiltRebuildsWhenLabelStale(t *testing.T) {
	// The image "exists" (ImageExists succeeds) but its label doesn't match
	// the current content hash, so EnsureBuilt must rebuild with the fresh
	// hash label rather than skip.
	r, argv := fakeRuntime(t, "stale-hash-from-old-build\n", false)
	if err := EnsureBuilt(r); err != nil {
		t.Fatalf("EnsureBuilt: %v", err)
	}
	got := strings.Join(argv(), " ")
	if !strings.HasPrefix(got, "build ") {
		t.Fatalf("argv = %q, want a rebuild when label is stale", got)
	}
	if !strings.Contains(got, "--target base") || !strings.Contains(got, "-t "+DefaultTag) {
		t.Errorf("argv = %q, want --target base and -t %s", got, DefaultTag)
	}
	if !strings.Contains(got, imageVersionLabel+"="+contentHash()) {
		t.Errorf("argv = %q, want the fresh content hash as the label", got)
	}
}

func TestEnsureDevenvBuiltRebuildsWhenLabelStale(t *testing.T) {
	r, argv := fakeRuntime(t, "stale-hash-from-old-build\n", false)
	if err := EnsureDevenvBuilt(r); err != nil {
		t.Fatalf("EnsureDevenvBuilt: %v", err)
	}
	got := strings.Join(argv(), " ")
	if !strings.HasPrefix(got, "build ") {
		t.Fatalf("argv = %q, want a rebuild when label is stale", got)
	}
	if !strings.Contains(got, "--target devenv") || !strings.Contains(got, "-t "+DevenvTag) {
		t.Errorf("argv = %q, want --target devenv and -t %s", got, DevenvTag)
	}
	if !strings.Contains(got, imageVersionLabel+"="+contentHash()) {
		t.Errorf("argv = %q, want the fresh content hash as the label", got)
	}
}

func TestDevenvTagDistinctFromDefaultTag(t *testing.T) {
	if DevenvTag == DefaultTag {
		t.Fatalf("DevenvTag must differ from DefaultTag, got %q for both", DevenvTag)
	}
	if !strings.Contains(DevenvTag, "devenv") {
		t.Errorf("DevenvTag = %q, want it to mention devenv", DevenvTag)
	}
}
