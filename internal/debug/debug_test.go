package debug

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestLogf(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		verbose    bool
		wantOutput string
	}{
		{name: "outputs when env debug enabled", enabled: true, wantOutput: "test message: hello\n"},
		{name: "outputs when verbose enabled", verbose: true, wantOutput: "test message: hello\n"},
		{name: "no output when disabled", wantOutput: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldEnabled := enabled
			oldVerbose := verboseMode
			oldStderr := os.Stderr
			defer func() {
				enabled = oldEnabled
				verboseMode = oldVerbose
				os.Stderr = oldStderr
			}()

			enabled = tt.enabled
			verboseMode = tt.verbose

			r, w, _ := os.Pipe()
			os.Stderr = w

			Logf("test message: %s\n", "hello")

			_ = w.Close()
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)

			if got := buf.String(); got != tt.wantOutput {
				t.Errorf("Logf() output = %q, want %q", got, tt.wantOutput)
			}
		})
	}
}

func TestSetVerbose(t *testing.T) {
	oldVerbose := verboseMode
	defer func() { verboseMode = oldVerbose }()

	SetVerbose(false)
	if verboseMode {
		t.Fatal("verboseMode should be false after SetVerbose(false)")
	}

	SetVerbose(true)
	if !verboseMode {
		t.Fatal("verboseMode should be true after SetVerbose(true)")
	}
}

func TestSetQuietAndIsQuiet(t *testing.T) {
	oldQuiet := quietMode
	defer func() { quietMode = oldQuiet }()

	SetQuiet(false)
	if IsQuiet() {
		t.Fatal("IsQuiet() should be false after SetQuiet(false)")
	}

	SetQuiet(true)
	if !IsQuiet() {
		t.Fatal("IsQuiet() should be true after SetQuiet(true)")
	}
}
