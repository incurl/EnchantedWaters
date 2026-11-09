package main

import "testing"

// TestSplitCommand verifies that the argument list is split safely.
//
// The regression this test guards against: slicing os.Args[2:] when the
// program is started without any argument panics with
// "slice bounds out of range [2:1]".
func TestSplitCommand(t *testing.T) {
	cases := []struct {
		name        string
		argv        []string
		wantCommand string
		wantArgs    int
	}{
		{"no arguments", []string{"game"}, "run", 0},
		{"only the program", []string{"/usr/bin/game"}, "run", 0},
		{"a subcommand", []string{"game", "validate"}, "validate", 0},
		{"a subcommand with arguments", []string{"game", "import", "--raw=."}, "import", 1},
		{"several arguments", []string{"game", "run", "--config=x", "--port=y"}, "run", 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command, args := splitCommand(tc.argv)
			if command != tc.wantCommand {
				t.Errorf("the command is %q, want %q", command, tc.wantCommand)
			}
			if len(args) != tc.wantArgs {
				t.Errorf("%d arguments were returned, want %d", len(args), tc.wantArgs)
			}
		})
	}
}

// TestSplitCommandNeverPanics verifies that no argument list can make the
// splitter panic.
func TestSplitCommandNeverPanics(t *testing.T) {
	// Every prefix of a realistic argument list is exercised.
	full := []string{"game", "import", "--raw=/tmp", "--out=/tmp/out", "--spec=/tmp/spec"}
	for i := 0; i <= len(full); i++ {
		argv := full[:i]
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("the splitter panicked for %v: %v", argv, r)
				}
			}()
			command, args := splitCommand(argv)
			if command == "" {
				t.Errorf("the command is empty for %v", argv)
			}
			_ = args
		}()
	}
}

// TestParseFlags verifies the parsing of the --key=value arguments.
func TestParseFlags(t *testing.T) {
	flags := parseFlags([]string{"--raw=/tmp/raw", "--out=/tmp/out", "--verbose=0", "positional"})
	if flags["raw"] != "/tmp/raw" {
		t.Errorf("the raw flag is %q, want /tmp/raw", flags["raw"])
	}
	if flags["out"] != "/tmp/out" {
		t.Errorf("the out flag is %q, want /tmp/out", flags["out"])
	}
	if flags["verbose"] != "0" {
		t.Errorf("the verbose flag is %q, want 0", flags["verbose"])
	}
	// A flag without a value is ignored instead of producing an empty
	// entry.
	if _, ok := flags["positional"]; ok {
		t.Error("a positional argument was parsed as a flag")
	}
}
