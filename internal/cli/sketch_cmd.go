package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/download"
	idx "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/index"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/output"
	"github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/sketch"
)

const (
	sketchSubdir  = "sketch"
	sketchSkmName = "atb_sketchlib.skm"
	sketchSkdName = "atb_sketchlib.skd"

	sketchSkmURL = "https://osf.io/download/nwfkc/"
	sketchSkdURL = "https://osf.io/download/92qmr/"
	sketchSkmMD5 = ""
	sketchSkdMD5 = ""
)

func sketchDir(dir string) string {
	return filepath.Join(dir, sketchSubdir)
}

func sketchSkmPath(dir string) string {
	return filepath.Join(sketchDir(dir), sketchSkmName)
}

func sketchDbPrefix(dir string) string {
	return filepath.Join(sketchDir(dir), "atb_sketchlib")
}

func sketchDbExists(dir string) bool {
	skm := filepath.Join(sketchDir(dir), sketchSkmName)
	skd := filepath.Join(sketchDir(dir), sketchSkdName)
	_, err1 := os.Stat(skm)
	_, err2 := os.Stat(skd)
	return err1 == nil && err2 == nil
}

func newSketchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sketch",
		Short: "Find closest ATB genomes using sketch distances",
		Long: `Find the closest genomes in the AllTheBacteria database to your input
sequences using MinHash sketch distances (via sketchlib).

Requires sketchlib (Linux/macOS only). Run 'atb sketch install' to download it.`,
	}

	cmd.AddCommand(newSketchInstallCmd())
	cmd.AddCommand(newSketchFetchCmd())
	cmd.AddCommand(newSketchQueryCmd())
	cmd.AddCommand(newSketchInfoCmd())

	return cmd
}

func newSketchInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Download the sketchlib binary (Linux/macOS only)",
		Long: `Download the sketchlib binary from GitHub releases and install it
alongside the atb binary. This is required for 'atb sketch query'.

Not available on Windows.`,
		Example: `  atb sketch install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if already installed
			if path, err := sketch.FindBinary(); err == nil {
				fmt.Fprintf(os.Stderr, "sketchlib already installed at %s\n", path)
				return nil
			}

			return sketch.InstallBinary(func(msg string) {
				fmt.Fprintln(os.Stderr, msg)
			})
		},
	}
}

func newSketchFetchCmd() *cobra.Command {
	var (
		force  bool
		verify bool
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download the ATB sketch database from OSF",
		Long: `Download the AllTheBacteria sketch database (~4.2 GB) from OSF.
This is required before running 'atb sketch query'.`,
		Example: `  atb sketch fetch
  atb sketch fetch --force --verify`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			sDir := sketchDir(dir)

			if !force && sketchDbExists(dir) {
				fmt.Fprintln(os.Stderr, "Sketch database already exists. Use --force to re-download.")
				return nil
			}

			fmt.Fprintln(os.Stderr, "Downloading ATB sketch database (~4.2 GB)...")

			tasks := []download.FileTask{
				{URL: sketchSkmURL, Filename: sketchSkmName},
				{URL: sketchSkdURL, Filename: sketchSkdName},
			}
			if verify {
				tasks[0].MD5 = sketchSkmMD5
				tasks[1].MD5 = sketchSkdMD5
			}

			dl := download.New(download.Config{
				OutputDir:  sDir,
				Parallel:   2,
				MaxRetries: 3,
			})

			dl.OnProgress = func(filename string, written, total int64) {
				if total > 0 {
					pct := float64(written) / float64(total) * 100
					fmt.Fprintf(os.Stderr, "\r  %s: %.1f%% (%s / %s)",
						filename, pct, formatSize(written), formatSize(total))
				} else {
					fmt.Fprintf(os.Stderr, "\r  %s: %s", filename, formatSize(written))
				}
			}

			result := dl.DownloadAllFiles(tasks)

			fmt.Fprintf(os.Stderr, "\r\033[K")
			fmt.Fprintf(os.Stderr, "Completed: %d/%d  Bytes: %s\n",
				result.Completed, result.Total, formatSize(result.Bytes))

			if result.Failed > 0 {
				for _, e := range result.Errors {
					fmt.Fprintf(os.Stderr, "  error: %s\n", e.Error)
				}
				return fmt.Errorf("%d download(s) failed", result.Failed)
			}

			fmt.Fprintln(os.Stderr, "Sketch database ready.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "re-download even if database exists")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify MD5 checksums after download")

	return cmd
}

func newSketchQueryCmd() *cobra.Command {
	var (
		fileList string
		knn      int
		threads  int
		format   string
		raw      bool
	)

	cmd := &cobra.Command{
		Use:   "query <fasta> [fasta...]",
		Short: "Find closest ATB genomes for your input sequences",
		Long: `Sketch your input FASTA file(s) and find the closest matches in the
AllTheBacteria sketch database. Results include ANI (Average Nucleotide
Identity) and metadata from the local ATB index.`,
		Example: `  # Find 10 closest genomes (default)
  atb sketch query my_genome.fasta

  # Top 50 closest
  atb sketch query my_genome.fasta --knn 50

  # Multiple input files
  atb sketch query sample1.fasta sample2.fasta

  # Batch from file list (one path per line)
  atb sketch query -f input_list.txt

  # Raw sketchlib distances without metadata enrichment
  atb sketch query my_genome.fasta --raw`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && fileList == "" {
				return cmd.Help()
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			if _, err := sketch.FindBinary(); err != nil {
				return err
			}

			if !sketchDbExists(dir) {
				return fmt.Errorf("sketch database not found. Run 'atb sketch fetch' first")
			}

			inputs := args
			if fileList != "" {
				lines, err := readLines(fileList)
				if err != nil {
					return fmt.Errorf("reading file list: %w", err)
				}
				inputs = append(inputs, lines...)
			}
			if len(inputs) == 0 {
				return fmt.Errorf("no input files provided")
			}

			for _, f := range inputs {
				if _, err := os.Stat(f); err != nil {
					return fmt.Errorf("input file: %w", err)
				}
			}

			dbInfo, err := sketch.Info(sketchSkmPath(dir))
			if err != nil {
				return fmt.Errorf("reading sketch database: %w", err)
			}
			if len(dbInfo.KmerSizes) == 0 {
				return fmt.Errorf("sketch database has no k-mer sizes")
			}

			fmt.Fprintf(os.Stderr, "Sketching %d input file(s)...\n", len(inputs))

			tmpDir, queryPrefix, err := sketch.SketchQuery(inputs, dbInfo.KmerSizes, threads)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			fmt.Fprintf(os.Stderr, "Querying ATB database (%d genomes)...\n", dbInfo.Samples)

			kmer := dbInfo.KmerSizes[len(dbInfo.KmerSizes)-1]
			matches, err := sketch.QueryDist(sketchDbPrefix(dir), queryPrefix, kmer, threads, knn)
			if err != nil {
				return err
			}

			if len(matches) == 0 {
				fmt.Fprintln(os.Stderr, "No matches found.")
				return nil
			}

			if matches[0].ANI < 0.80 {
				fmt.Fprintf(os.Stderr, "Warning: closest match has low ANI (%.1f%%) — query may not be bacterial or may be a novel species.\n",
					matches[0].ANI*100)
			}

			fmt.Fprintf(os.Stderr, "%d match(es) found.\n\n", len(matches))

			if raw {
				return printRawMatches(cmd, matches, format)
			}

			return printEnrichedMatches(cmd, matches, dir, format)
		},
	}

	cmd.Flags().StringVarP(&fileList, "file-list", "f", "", "file with one FASTA path per line")
	cmd.Flags().IntVar(&knn, "knn", 10, "number of closest matches to return")
	cmd.Flags().IntVar(&threads, "threads", 0, "CPU threads (default: all available)")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table (default: auto)")
	cmd.Flags().BoolVar(&raw, "raw", false, "output raw distances without metadata enrichment")

	return cmd
}

func newSketchInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info",
		Short:   "Show information about the local sketch database",
		Example: `  atb sketch info`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			if !sketchDbExists(dir) {
				return fmt.Errorf("sketch database not found. Run 'atb sketch fetch' first")
			}

			info, err := sketch.Info(sketchSkmPath(dir))
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Sketch database: %s\n", sketchSkmPath(dir))
			fmt.Fprintf(w, "Samples:         %d\n", info.Samples)
			fmt.Fprintf(w, "K-mer sizes:     %v\n", info.KmerSizes)
			fmt.Fprintf(w, "Sketch size:     %d\n", info.SketchSize)

			sDir := sketchDir(dir)
			var totalSize int64
			for _, name := range []string{sketchSkmName, sketchSkdName} {
				if fi, err := os.Stat(filepath.Join(sDir, name)); err == nil {
					totalSize += fi.Size()
				}
			}
			fmt.Fprintf(w, "Database size:   %s\n", formatSize(totalSize))

			return nil
		},
	}
}

func printRawMatches(cmd *cobra.Command, matches []sketch.Match, format string) error {
	resolved := output.ResolveFormat(format)
	columns := []string{"query", "sample_accession", "ani"}
	rows := make([]output.Row, len(matches))
	for i, m := range matches {
		rows[i] = output.Row{
			"query":            m.QueryName,
			"sample_accession": m.RefName,
			"ani":              fmt.Sprintf("%.4f", m.ANI),
		}
	}
	return output.Format(cmd.OutOrStdout(), rows, columns, resolved)
}

func printEnrichedMatches(cmd *cobra.Command, matches []sketch.Match, dir, format string) error {
	resolved := output.ResolveFormat(format)

	var db *idx.DB
	if idx.Exists(dir) {
		var err error
		db, err = idx.Open(dir)
		if err == nil {
			defer db.Close()
		}
	}

	columns := []string{"query", "sample_accession", "ani", "species", "N50", "completeness", "mlst_st"}
	rows := make([]output.Row, len(matches))
	for i, m := range matches {
		row := output.Row{
			"query":            m.QueryName,
			"sample_accession": m.RefName,
			"ani":              fmt.Sprintf("%.4f", m.ANI),
			"species":          "-",
			"N50":              "-",
			"completeness":     "-",
			"mlst_st":          "-",
		}

		if db != nil {
			if info, err := db.InfoRow(m.RefName); err == nil {
				if v := info["sylph_species"]; v != "" {
					row["species"] = v
				}
				if v := info["N50"]; v != "" {
					row["N50"] = v
				}
				if v := info["Completeness_General"]; v != "" {
					row["completeness"] = v
				}
				if v := info["mlst_st"]; v != "" {
					row["mlst_st"] = v
				}
			}
		}

		rows[i] = row
	}

	return output.Format(cmd.OutOrStdout(), rows, columns, resolved)
}
