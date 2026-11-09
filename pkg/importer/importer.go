package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Options configures an import run.
type Options struct {
	// RawDir is the directory that holds the original asset files.
	//
	// The directory is supplied by the user; nothing of it is ever
	// committed to the repository.
	RawDir string
	// SpecDir is the directory that holds the format specifications.
	SpecDir string
	// OutDir is the directory the converted tables are written to.
	OutDir string
	// LogPath is the file the detailed audit log is written to.
	LogPath string
	// Verbose enables the per-record logging.
	Verbose bool
}

// Result summarises an import run.
type Result struct {
	// Tables holds the number of converted records per table.
	Tables map[string]int
	// Files holds the paths of the written files.
	Files []string
	// Problems holds the human readable notes about the conversion.
	Problems []string
}

// Report returns a human readable summary of the run.
func (r *Result) Report() string {
	out := "import finished:\n"
	names := make([]string, 0, len(r.Tables))
	for name := range r.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out += fmt.Sprintf("  %-24s %d records\n", name, r.Tables[name])
	}
	for _, p := range r.Problems {
		out += "  problem: " + p + "\n"
	}
	return out
}

// Run executes the import of every specification of the spec directory.
//
// The function is the entry point of the "import" subcommand.
func Run(opts Options, log Logger) (*Result, error) {
	result := &Result{Tables: make(map[string]int)}

	if opts.RawDir == "" {
		return nil, fmt.Errorf("no raw directory given (--raw=...)")
	}
	if _, err := os.Stat(opts.RawDir); err != nil {
		return nil, fmt.Errorf("raw directory %s: %w", opts.RawDir, err)
	}
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join("assets", "config")
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	specs, err := LoadSpecs(opts.SpecDir)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("no specification found in %s", opts.SpecDir)
	}

	for _, spec := range specs {
		log.Info("=== importing %s: %s", spec.Name, spec.Description)
		if err := importSpec(spec, opts, log, result); err != nil {
			// A failing specification does not abort the whole run: the
			// other tables are still converted, and the problems are
			// reported at the end.
			result.Problems = append(result.Problems, err.Error())
			log.Problem("%v", err)
		}
	}
	return result, nil
}

// importSpec converts one specification.
func importSpec(spec *Spec, opts Options, log Logger, result *Result) error {
	// --- open the source ---
	reader, closer, err := openSource(spec, opts)
	if err != nil {
		return err
	}
	defer closer()

	for _, table := range spec.Tables {
		converted, err := ConvertTable(reader, spec, table, log)
		if err != nil {
			result.Problems = append(result.Problems, err.Error())
			log.Problem("%v", err)
			continue
		}
		for _, p := range converted.Problems {
			result.Problems = append(result.Problems, p)
			log.Problem("%s", p)
		}
		if err := writeTable(converted, opts.OutDir, log); err != nil {
			result.Problems = append(result.Problems, err.Error())
			log.Problem("%v", err)
			continue
		}
		result.Tables[table.Name] = len(converted.Rows)
		log.Info("table %s: %d records written", table.Name, len(converted.Rows))
	}
	return nil
}

// openSource opens the data source of a specification.
//
// It returns a reader over the data section and a closer that releases
// the underlying resources.
func openSource(spec *Spec, opts Options) (*Reader, func(), error) {
	// --- the file may be inside an archive ---
	if spec.Source.Archive != "" {
		archivePath := filepath.Join(opts.RawDir, spec.Source.Archive)
		archive, err := OpenArchive(archivePath)
		if err != nil {
			return nil, nil, err
		}
		entry, err := archive.Find(spec.Source.Path)
		if err != nil {
			archive.Close()
			return nil, nil, err
		}
		stream, size, err := archive.Open(entry)
		if err != nil {
			archive.Close()
			return nil, nil, err
		}
		length := spec.Source.Length
		if length <= 0 || length > size {
			length = size
		}
		reader := NewSectionReader(stream, spec.Source.Offset, length)
		return reader, func() {
			stream.Close()
			archive.Close()
		}, nil
	}

	// --- a plain file ---
	path := filepath.Join(opts.RawDir, spec.Source.Path)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	length := spec.Source.Length
	if length <= 0 || spec.Source.Offset+length > info.Size() {
		length = info.Size() - spec.Source.Offset
	}
	if length < 0 {
		f.Close()
		return nil, nil, fmt.Errorf("invalid data section in %s", path)
	}
	reader := NewReader(f, spec.Source.Offset, length)
	return reader, func() { f.Close() }, nil
}

// writeTable writes a converted table as a CSV and a JSON file.
//
// The two formats hold exactly the same data; the CSV is the canonical
// one for the runtime, the JSON is the convenient one for the tools.
func writeTable(t *Table, outDir string, log Logger) error {
	csvPath := filepath.Join(outDir, t.Name+".csv")
	f, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", csvPath, err)
	}
	if err := WriteCSV(f, t); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", csvPath, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	log.Info("wrote %s", csvPath)

	// --- the JSON variant ---
	jsonPath := filepath.Join(outDir, t.Name+".json")
	records := make([]map[string]string, 0, len(t.Rows))
	for _, row := range t.Rows {
		rec := make(map[string]string, len(t.Columns))
		for i, col := range t.Columns {
			if i < len(row.Cells) {
				rec[col] = row.Cells[i]
			}
		}
		records = append(records, rec)
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", jsonPath, err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", jsonPath, err)
	}
	log.Info("wrote %s", jsonPath)
	return nil
}

// copyStream copies a stream into a writer, returning the number of
// bytes that were copied.
func copyStream(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
