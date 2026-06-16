package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mnemoo/mlrshift/internal/cli"
)

// TestMagicRTPLockOnRealLUT is the end-to-end guarantee the user cares about:
// on a real base-game LUT, `magic` hits a feasible target while keeping RTP
// locked, and clearly does NOT claim a lock when the target is infeasible.
func TestMagicRTPLockOnRealLUT(t *testing.T) {
	run := func(args ...string) string {
		var buf bytes.Buffer
		if code := cli.Run(args, &buf, &buf); code != 0 {
			t.Fatalf("exit %d: %s", code, buf.String())
		}
		return buf.String()
	}

	feasible := run("magic", "--target-n", "250", "testdata/base.csv")
	if !strings.Contains(feasible, "RTP lock:       ✓ held") {
		t.Errorf("feasible magic should hold RTP:\n%s", feasible)
	}
	if !strings.Contains(feasible, "1 in 250.00") {
		t.Errorf("feasible magic should hit target:\n%s", feasible)
	}

	infeasible := run("magic", "--target-n", "1000", "testdata/base.csv")
	if !strings.Contains(infeasible, "RTP lock:       ✗ NOT held") {
		t.Errorf("infeasible magic must flag the broken lock:\n%s", infeasible)
	}
}

// TestDefaultCommandIsMagic verifies that omitting the command runs `magic`:
// `mlrshift <file> --target-n N` is identical to `mlrshift magic <file> ...`.
func TestDefaultCommandIsMagic(t *testing.T) {
	var withWord, withoutWord bytes.Buffer
	if code := cli.Run([]string{"magic", "--target-n", "250", "testdata/base.csv"}, &withWord, &withWord); code != 0 {
		t.Fatalf("magic exit: %s", withWord.String())
	}
	if code := cli.Run([]string{"--target-n", "250", "testdata/base.csv"}, &withoutWord, &withoutWord); code != 0 {
		t.Fatalf("default exit: %s", withoutWord.String())
	}
	if withWord.String() != withoutWord.String() {
		t.Errorf("default command should match `magic`:\n--- magic ---\n%s\n--- default ---\n%s", withWord.String(), withoutWord.String())
	}
}
