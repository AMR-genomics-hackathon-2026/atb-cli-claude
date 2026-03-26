package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	idx "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/index"
	pq "github.com/AMR-genomics-hackathon-2026/atb-cli-claude/internal/parquet"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <sample_accession>",
		Short: "Show detailed information for a sample",
		Example: `  # Show all available info for a sample
  atb info SAMD00000355

  # Look up by run accession
  atb info SRR11427802`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			accession := args[0]

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			if err := ensureDatabase(dir); err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			// Try SQLite index first (instant single-sample lookup).
			if idx.Exists(dir) {
				if db, openErr := idx.Open(dir); openErr == nil {
					defer db.Close()
					if row, rowErr := db.InfoRow(accession); rowErr == nil {
						fmt.Fprintln(w, "=== Assembly ===")
						fmt.Fprintf(w, "  sample_accession:   %s\n", row["sample_accession"])
						fmt.Fprintf(w, "  run_accession:      %s\n", row["run_accession"])
						fmt.Fprintf(w, "  assembly_accession: %s\n", row["assembly_accession"])
						fmt.Fprintf(w, "  sylph_species:      %s\n", row["sylph_species"])
						fmt.Fprintf(w, "  scientific_name:    %s\n", row["scientific_name"])
						fmt.Fprintf(w, "  hq_filter:          %s\n", row["hq_filter"])
						fmt.Fprintf(w, "  dataset:            %s\n", row["dataset"])
						fmt.Fprintf(w, "  asm_fasta_on_osf:   %s\n", row["asm_fasta_on_osf"])
						fmt.Fprintf(w, "  aws_url:            %s\n", row["aws_url"])
						fmt.Fprintf(w, "  osf_tarball_url:    %s\n", row["osf_tarball_url"])
						fmt.Fprintln(w)

						if row["N50"] != "" {
							fmt.Fprintln(w, "=== Assembly Stats ===")
							fmt.Fprintf(w, "  total_length: %s\n", row["total_length"])
							fmt.Fprintf(w, "  number:       %s\n", row["number"])
							fmt.Fprintf(w, "  N50:          %s\n", row["N50"])
							fmt.Fprintf(w, "  N90:          %s\n", row["N90"])
							fmt.Fprintln(w)
						}

						if row["Completeness_General"] != "" {
							fmt.Fprintln(w, "=== CheckM2 Quality ===")
							fmt.Fprintf(w, "  completeness_general: %s\n", row["Completeness_General"])
							fmt.Fprintf(w, "  contamination:        %s\n", row["Contamination"])
							fmt.Fprintf(w, "  genome_size:          %s\n", row["Genome_Size"])
							fmt.Fprintf(w, "  gc_content:           %s\n", row["GC_Content"])
							fmt.Fprintln(w)
						}
						return nil
					}
				}
			}

			found := false

			// Assembly info
			assemblyPath := filepath.Join(dir, "assembly.parquet")
			if _, err := os.Stat(assemblyPath); err == nil {
				rows, err := pq.ReadFiltered[pq.AssemblyRow](assemblyPath, func(r pq.AssemblyRow) bool {
					return r.SampleAccession == accession
				})
				if err != nil {
					fmt.Fprintf(w, "assembly: error reading: %v\n", err)
				} else if len(rows) > 0 {
					found = true
					a := rows[0]
					fmt.Fprintln(w, "=== Assembly ===")
					fmt.Fprintf(w, "  sample_accession:   %s\n", a.SampleAccession)
					fmt.Fprintf(w, "  run_accession:      %s\n", a.RunAccession)
					fmt.Fprintf(w, "  assembly_accession: %s\n", a.AssemblyAccession)
					fmt.Fprintf(w, "  sylph_species:      %s\n", a.SylphSpecies)
					fmt.Fprintf(w, "  scientific_name:    %s\n", a.ScientificName)
					fmt.Fprintf(w, "  hq_filter:          %s\n", a.HQFilter)
					fmt.Fprintf(w, "  dataset:            %s\n", a.Dataset)
					fmt.Fprintf(w, "  asm_fasta_on_osf:   %d\n", a.AsmFastaOnOSF)
					fmt.Fprintf(w, "  aws_url:            %s\n", a.AWSUrl)
					fmt.Fprintf(w, "  osf_tarball_url:    %s\n", a.OSFTarballURL)
					fmt.Fprintln(w)
				}
			}

			// Assembly stats
			statsPath := filepath.Join(dir, "assembly_stats.parquet")
			if _, err := os.Stat(statsPath); err == nil {
				rows, err := pq.ReadFiltered[pq.AssemblyStatsRow](statsPath, func(r pq.AssemblyStatsRow) bool {
					return r.SampleAccession == accession
				})
				if err != nil {
					fmt.Fprintf(w, "assembly_stats: error reading: %v\n", err)
				} else if len(rows) > 0 {
					s := rows[0]
					fmt.Fprintln(w, "=== Assembly Stats ===")
					fmt.Fprintf(w, "  total_length: %d\n", s.TotalLength)
					fmt.Fprintf(w, "  number:       %d\n", s.Number)
					fmt.Fprintf(w, "  mean_length:  %.2f\n", s.MeanLength)
					fmt.Fprintf(w, "  longest:      %d\n", s.Longest)
					fmt.Fprintf(w, "  shortest:     %d\n", s.Shortest)
					fmt.Fprintf(w, "  N50:          %d\n", s.N50)
					fmt.Fprintf(w, "  N90:          %d\n", s.N90)
					fmt.Fprintln(w)
				}
			}

			// CheckM2 quality
			checkm2Path := filepath.Join(dir, "checkm2.parquet")
			if _, err := os.Stat(checkm2Path); err == nil {
				rows, err := pq.ReadFiltered[pq.CheckM2Row](checkm2Path, func(r pq.CheckM2Row) bool {
					return r.SampleAccession == accession
				})
				if err != nil {
					fmt.Fprintf(w, "checkm2: error reading: %v\n", err)
				} else if len(rows) > 0 {
					c := rows[0]
					fmt.Fprintln(w, "=== CheckM2 Quality ===")
					fmt.Fprintf(w, "  completeness_general:  %.4f\n", c.CompletenessGeneral)
					fmt.Fprintf(w, "  completeness_specific: %.4f\n", c.CompletenessSpecific)
					fmt.Fprintf(w, "  contamination:         %.4f\n", c.Contamination)
					fmt.Fprintf(w, "  genome_size:           %.0f\n", c.GenomeSize)
					fmt.Fprintf(w, "  gc_content:            %.4f\n", c.GCContent)
					fmt.Fprintln(w)
				}
			}

			// ENA metadata (optional)
			enaPath := filepath.Join(dir, "ena_20250506.parquet")
			if _, err := os.Stat(enaPath); err == nil {
				rows, err := pq.ReadFiltered[pq.ENARow](enaPath, func(r pq.ENARow) bool {
					return r.SampleAccession == accession
				})
				if err != nil {
					fmt.Fprintf(w, "ena: error reading: %v\n", err)
				} else if len(rows) > 0 {
					e := rows[0]
					fmt.Fprintln(w, "=== ENA Metadata ===")
					fmt.Fprintf(w, "  country:             %s\n", e.Country)
					fmt.Fprintf(w, "  collection_date:     %s\n", e.CollectionDate)
					fmt.Fprintf(w, "  instrument_platform: %s\n", e.InstrumentPlatform)
					fmt.Fprintf(w, "  instrument_model:    %s\n", e.InstrumentModel)
					fmt.Fprintf(w, "  read_count:          %d\n", e.ReadCount)
					fmt.Fprintf(w, "  base_count:          %d\n", e.BaseCount)
					fmt.Fprintf(w, "  library_strategy:    %s\n", e.LibraryStrategy)
					fmt.Fprintf(w, "  study_accession:     %s\n", e.StudyAccession)
					fmt.Fprintf(w, "  fastq_ftp:           %s\n", e.FastqFTP)
					fmt.Fprintln(w)
				}
			} else {
				fmt.Fprintln(w, "ENA table not downloaded. Run 'atb fetch --all' to get all tables.")
			}

			if !found {
				return fmt.Errorf("sample %q not found in assembly table", accession)
			}

			return nil
		},
	}
}
