package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain lets this test binary double as a fake runtime CLI: when invoked
// with AICO_TEST_FAKE_BIN=1 it records its own argv to the file named by
// AICO_TEST_FAKE_BIN_RECORD and exits 0, instead of running tests. This lets
// Commit/BuildWithDockerfile be asserted on real argv without depending on a
// real docker/podman binary or a shell script (which wouldn't run on
// Windows) -- the fake "binary" is just this same compiled test binary.
func TestMain(m *testing.M) {
	if os.Getenv("AICO_TEST_FAKE_BIN") == "1" {
		record := os.Getenv("AICO_TEST_FAKE_BIN_RECORD")
		_ = os.WriteFile(record, []byte(strings.Join(os.Args[1:], " ")), 0o644)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeRuntime returns a Runtime whose Bin is this test binary re-invoked as a
// stub, plus a function to read back the argv it was last called with.
func fakeRuntime(t *testing.T) (*Runtime, func() []string) {
	t.Helper()
	record := filepath.Join(t.TempDir(), "argv")
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("AICO_TEST_FAKE_BIN", "1")
	t.Setenv("AICO_TEST_FAKE_BIN_RECORD", record)
	r := &Runtime{Bin: self}
	return r, func() []string {
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatalf("read record: %v", err)
		}
		return strings.Fields(string(data))
	}
}

func TestCommitArgv(t *testing.T) {
	r, argv := fakeRuntime(t)
	if err := r.Commit("mycontainer", "my/tag:v1"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	want := []string{"commit", "mycontainer", "my/tag:v1"}
	got := argv()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestBuildWithDockerfileArgv(t *testing.T) {
	r, argv := fakeRuntime(t)
	if err := r.BuildWithDockerfile("t/x", "/tmp/aico-bake/Dockerfile", "/proj"); err != nil {
		t.Fatalf("BuildWithDockerfile: %v", err)
	}
	want := []string{"build", "-t", "t/x", "-f", "/tmp/aico-bake/Dockerfile", "/proj"}
	got := argv()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestCreateArgv(t *testing.T) {
	r, argv := fakeRuntime(t)
	if _, err := r.Create("--name", "foo", "myimage"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := []string{"create", "--name", "foo", "myimage"}
	got := argv()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}
