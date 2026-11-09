package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peter/enchantedwaters/pkg/importer"
)

// runImport executes the asset import conversion.
//
// The import is a development tool: it reads the raw asset files that
// the user supplies from their own disk and writes the converted tables
// into the assets directory of the project. The game runtime never calls
// any of this code.
func runImport(args []string) error {
	flags := parseFlags(args)

	rawDir := flags["raw"]
	if rawDir == "" {
		return fmt.Errorf("no raw directory given (--raw=PATH_TO_RAW_ASSET)")
	}
	outDir := flags["out"]
	if outDir == "" {
		outDir = filepath.Join("assets", "config")
	}
	specDir := flags["spec"]
	if specDir == "" {
		specDir = defaultSpecDir()
	}
	logPath := flags["log"]
	if logPath == "" {
		logPath = filepath.Join(outDir, "import.log")
	}
	verbose := flags["verbose"] != "0" && flags["quiet"] == ""

	// The console logger carries the summary; the file logger carries the
	// detailed audit trail of every parsed record.
	console := importer.NewConsoleLogger(os.Stdout, false)
	// The directory of the log is created first: the log may live
	// outside of the output directory.
	if dir := filepath.Dir(logPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create log directory %s: %w", dir, err)
		}
	}
	fileLog, err := importer.NewFileLogger(logPath, console, verbose)
	if err != nil {
		return fmt.Errorf("create log %s: %w", logPath, err)
	}
	defer fileLog.Close()

	fmt.Printf("importing from %s using the specifications of %s\n", rawDir, specDir)
	fmt.Printf("the output is written to %s, the audit log to %s\n\n", outDir, logPath)

	result, err := importer.Run(importer.Options{
		RawDir:  rawDir,
		SpecDir: specDir,
		OutDir:  outDir,
		LogPath: logPath,
		Verbose: verbose,
	}, fileLog)
	if err != nil {
		return err
	}

	fmt.Print("\n")
	fmt.Print(result.Report())

	// The validation step reports the problems that were found.
	if len(result.Problems) > 0 {
		fmt.Printf("\n%d problem(s) were found. The converted files must be\n", len(result.Problems))
		fmt.Print("reviewed by hand before they are committed.\n")
		return fmt.Errorf("import finished with %d problem(s)", len(result.Problems))
	}
	fmt.Print("\nThe converted files should be reviewed by hand before they are committed.\n")
	return nil
}

// defaultSpecDir returns the default location of the format
// specifications.
func defaultSpecDir() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(exe), "assets", "specs")
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return filepath.Join("assets", "specs")
}
