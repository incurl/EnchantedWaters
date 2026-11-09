// Command game is the entry point of the Enchanted Waters project.
//
// The binary supports three subcommands:
//
//	game run                 start the playable game
//	game import --raw=DIR    convert original asset files into the
//	                         project's CSV/JSON/PNG format
//	game validate            check the configuration tables
//
// The import subcommand is a development tool: it parses the asset files
// that the user supplies on their own disk and writes the converted data
// into the assets directory. The game itself never loads any original
// binary asset.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-logr/logr"

	"github.com/peter/enchantedwaters/internal/asset"
	"github.com/peter/enchantedwaters/internal/ecs"
	"github.com/peter/enchantedwaters/internal/event"
	"github.com/peter/enchantedwaters/internal/ui"
	"github.com/peter/enchantedwaters/internal/world"
	"github.com/peter/enchantedwaters/pkg/etl"
)

func main() {
	// The subcommand is the first argument; the default is "run".
	command, args := splitCommand(os.Args)

	switch command {
	case "run":
		if err := runGame(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "import":
		if err := runImport(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "derive":
		if err := runDerive(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "validate":
		if err := runValidate(args); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		printVersion()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", command)
		printUsage()
		os.Exit(2)
	}
}

// splitCommand separates the subcommand from its arguments.
//
// The helper is defensive about the length of the argument list: the
// program may be started without any argument at all, in which case the
// default subcommand is used and the argument list is empty. Slicing
// os.Args directly would panic in that case.
func splitCommand(argv []string) (command string, args []string) {
	// The default subcommand is "run".
	command = "run"
	args = []string{}
	if len(argv) < 2 {
		return command, args
	}
	command = argv[1]
	// The rest of the arguments belong to the subcommand.
	if len(argv) > 2 {
		args = argv[2:]
	}
	return command, args
}

// printVersion prints the version of the build.
func printVersion() {
	fmt.Printf("enchanted-waters\n")
	fmt.Printf("  go:      %s\n", runtime.Version())
	fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// printUsage prints the usage of the binary.
func printUsage() {
	fmt.Print(`enchanted-waters - a clean-room reimplementation of Uncharted Waters II

Usage:
  game run [--config=DIR] [--port=PORT_ID]   start the playable game
  game import --raw=PATH [--out=DIR]          convert original asset files
  game derive [--config=DIR]                 regenerate the playable tables
  game validate [--config=DIR]                check the configuration tables
  game version                               print the version information

The import subcommand is a development tool. It only reads the asset files
that you supply on your own disk; no original asset is bundled with the
binary.
`)
}

// runGame starts the playable game.
func runGame(args []string) error {
	flags := parseFlags(args)
	configDir := flags["config"]
	if configDir == "" {
		configDir = defaultConfigDir()
	}
	startPort := flags["port"]
	if startPort == "" {
		startPort = "ports.lisbon"
	}

	log := newLogger()
	log.Info("starting game", "config", configDir, "port", startPort)

	// --- load the static configuration layer (once) ---
	reg, err := asset.Load(configDir, log)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// --- create the dynamic state layer ---
	w := world.NewWorld(reg, 0xC0FFEE)
	if err := w.StartSession(startPort); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	// --- the event scripting engine ---
	// The scripts are optional: when the directory does not exist, the
	// events are resolved from their table driven effects only.
	scriptDir := filepath.Join(filepath.Dir(configDir), "scripts")
	if _, err := os.Stat(scriptDir); err == nil {
		w.SetScripting(event.NewScripting(scriptDir))
		log.Info("event scripting enabled", "dir", scriptDir)
	}

	log.Info("session started", "ports", len(reg.Ports), "commodities", len(reg.Commodities))

	// --- create the game ---
	game := ui.NewGame(w)

	// --- the screenshot ---
	// The screenshot is written at the end of the first frame.
	if shot := flags["shot"]; shot != "" {
		game.SetScreenshot(shot)
	}

	// --- the world overview ---
	if flags["world"] != "" {
		game.UI().ShowWorld()
	}

	// --- open a screen at the start ---
	// The flag is used by the development screenshots.
	if screen := flags["open"]; screen != "" {
		if !game.UI().OpenScreen(screen) {
			log.Info("the screen could not be opened", "screen", screen)
		}
	}

	// The window is created with an integer scaling factor and
	// nearest-neighbour filtering, which preserves the retro look.
	ebitenSetWindowProperties()

	return runEbiten(game)
}

// runDerive regenerates the playable tables from the imported reference
// data of the original game.
//
// It is a development tool: the layout of the world comes from the
// original game, the rules are the balance of this project.
func runDerive(args []string) error {
	flags := parseFlags(args)
	configDir := flags["config"]
	if configDir == "" {
		configDir = defaultConfigDir()
	}
	summary, err := etl.DeriveAll(configDir)
	if err != nil {
		return err
	}
	fmt.Println(summary)
	fmt.Println("review the generated tables before committing them.")
	return nil
}

// runValidate checks the configuration tables and reports the problems.
//
// The validation runs the standalone ETL checker (which verifies the
// structure of the CSV files) and then the runtime loader (which verifies
// the semantic consistency of the data set).
func runValidate(args []string) error {
	flags := parseFlags(args)
	configDir := flags["config"]
	if configDir == "" {
		configDir = defaultConfigDir()
	}
	log := newLogger()

	// --- the structural check ---
	report, err := etl.Validate(configDir)
	if err != nil {
		return fmt.Errorf("validate tables: %w", err)
	}
	fmt.Print("table check:\n")
	fmt.Print(report.String())

	// --- the semantic check ---
	reg, err := asset.Load(configDir, log)
	if err != nil {
		return err
	}
	fmt.Printf("\nruntime check: %d commodities, %d ports, %d ships, %d events, %d rumors\n",
		len(reg.Commodities), len(reg.Ports), len(reg.Ships), len(reg.Events), len(reg.Rumors))

	if !report.OK() {
		return fmt.Errorf("%d problem(s) found in the configuration tables", len(report.Problems))
	}
	fmt.Println("\nthe configuration is valid.")
	return nil
}

// parseFlags parses the --key=value arguments of a subcommand.
func parseFlags(args []string) map[string]string {
	out := make(map[string]string, 4)
	for _, arg := range args {
		if len(arg) < 3 || arg[:2] != "--" {
			continue
		}
		kv := arg[2:]
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				out[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return out
}

// defaultConfigDir returns the default location of the configuration
// tables.
//
// The tables live next to the binary in assets/config, so that the
// released binary can be shipped with its data directory.
func defaultConfigDir() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join("assets", "config")
	}
	dir := filepath.Join(filepath.Dir(exe), "assets", "config")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	// Fall back to the working directory, which is what happens when the
	// game is started from the repository.
	return filepath.Join("assets", "config")
}

// newLogger builds the structured logger of the application.
func newLogger() logr.Logger {
	return logr.Discard()
}

// ecsEntityID is referenced to keep the ecs import meaningful in the
// documentation of the entity identifiers used by the game.
type ecsEntityID = ecs.ID
