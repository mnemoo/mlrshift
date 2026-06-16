package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLUT(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "LUT.csv")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestFlagsAnyOrder: the file positional is accepted before or after flags, and
// --flag=value / --flag value both work.
func TestFlagsAnyOrder(t *testing.T) {
	lut := writeLUT(t, "1,1000,0\n2,1,0\n3,1,200\n")
	variants := [][]string{
		{"shift", "--target-n", "5", "--donor", "loss", lut},
		{"shift", lut, "--target-n", "5", "--donor", "loss"},
		{"shift", "--target-n=5", "--donor=loss", lut},
		{"shift", "--target-n", "5", lut, "--donor", "loss"},
	}
	for i, args := range variants {
		var out, errOut bytes.Buffer
		if code := Run(args, &out, &errOut); code != 0 {
			t.Errorf("variant %d exit %d: %s", i, code, errOut.String())
		}
		if !strings.Contains(out.String(), "MLR shift:") {
			t.Errorf("variant %d missing shift output: %s", i, out.String())
		}
	}
}

func TestErrors(t *testing.T) {
	lut := writeLUT(t, "1,1000,0\n2,1,0\n")
	cases := []struct {
		name string
		args []string
	}{
		{"no command", []string{}},
		{"unknown command", []string{"bogus"}},
		{"shift no target", []string{"shift", lut}},
		{"shift no file", []string{"shift", "--target-n", "5"}},
		{"unknown flag", []string{"shift", "--target-n", "5", "--nope", lut}},
		{"bad donor", []string{"shift", "--target-n", "5", "--donor", "5-1", lut}},
		{"bad number", []string{"shift", "--target-n", "abc", lut}},
		{"two files", []string{"shift", "--target-n", "5", lut, lut}},
		{"magic no target", []string{"magic", lut}},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		if code := Run(c.args, &out, &errOut); code == 0 {
			t.Errorf("%s: expected non-zero exit", c.name)
		}
	}
}

func TestSaveWritesFile(t *testing.T) {
	lut := writeLUT(t, "1,1000,0\n2,1,0\n3,1,200\n4,1,0\n5,1,0\n")
	var out, errOut bytes.Buffer
	// dry run must NOT change the file
	before, _ := os.ReadFile(lut)
	if code := Run([]string{"shift", "--target-n", "5", "--donor", "loss", lut}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	after, _ := os.ReadFile(lut)
	if string(before) != string(after) {
		t.Error("dry run modified the file")
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Error("expected dry-run notice")
	}
	// --save must change it
	out.Reset()
	if code := Run([]string{"shift", "--target-n", "5", "--donor", "loss", "--save", lut}, &out, &errOut); code != 0 {
		t.Fatalf("save exit %d: %s", code, errOut.String())
	}
	saved, _ := os.ReadFile(lut)
	if string(saved) == string(before) {
		t.Error("--save did not modify the file")
	}
	if !strings.Contains(out.String(), "Saved to:") {
		t.Error("expected Saved notice")
	}
}

func TestVersionAndHelp(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"help"}, {"shift", "--help"}, {"magic", "--help"}, {"inspect", "--help"}} {
		var out, errOut bytes.Buffer
		if code := Run(args, &out, &errOut); code != 0 {
			t.Errorf("%v: exit %d", args, code)
		}
		if out.Len() == 0 {
			t.Errorf("%v: no output", args)
		}
	}
}

func TestJSONOutput(t *testing.T) {
	lut := writeLUT(t, "1,1000,0\n2,1,0\n3,1,200\n4,1,0\n5,1,0\n")
	var out, errOut bytes.Buffer
	if code := Run([]string{"magic", "--target-n", "5", "--json", lut}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"rtp_lock_held"`) {
		t.Errorf("expected JSON with rtp_lock_held, got: %s", out.String())
	}
}
