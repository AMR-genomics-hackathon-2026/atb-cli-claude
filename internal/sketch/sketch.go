package sketch

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const BinaryName = "sketchlib"

type DatabaseInfo struct {
	Samples    int
	KmerSizes  []int
	SketchSize int
}

func FindBinary() (string, error) {
	path, err := exec.LookPath(BinaryName)
	if err != nil {
		return "", fmt.Errorf(`sketchlib not found in PATH.

Install from pre-built binary (Linux/macOS):
  https://github.com/bacpop/sketchlib.rust/releases/latest

Or via package managers:
  cargo install sketchlib
  conda install -c bioconda sketchlib

Note: sketch features are not available on Windows.`)
	}
	return path, nil
}

func Info(skmPath string) (*DatabaseInfo, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "info", skmPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sketchlib info: %w", err)
	}
	return ParseInfo(strings.NewReader(string(out)))
}

func ParseInfo(r io.Reader) (*DatabaseInfo, error) {
	info := &DatabaseInfo{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "n_samples":
			info.Samples, _ = strconv.Atoi(val)
		case "sketch_size":
			info.SketchSize, _ = strconv.Atoi(val)
		case "kmers":
			val = strings.Trim(val, "[] ")
			for _, s := range strings.Split(val, ",") {
				s = strings.TrimSpace(s)
				if n, err := strconv.Atoi(s); err == nil {
					info.KmerSizes = append(info.KmerSizes, n)
				}
			}
		}
	}
	return info, scanner.Err()
}

type Match struct {
	RefName   string
	QueryName string
	ANI       float64
}

func ParseDistOutput(r io.Reader) ([]Match, error) {
	var matches []Match
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "sketchlib done") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		ani, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}
		matches = append(matches, Match{
			RefName:   fields[0],
			QueryName: fields[1],
			ANI:       ani,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ANI > matches[j].ANI
	})
	return matches, scanner.Err()
}

func SketchQuery(inputs []string, kmerSizes []int, threads int) (tmpDir, prefix string, err error) {
	bin, err := FindBinary()
	if err != nil {
		return "", "", err
	}
	tmpDir, err = os.MkdirTemp("", "atb-sketch-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}
	prefix = filepath.Join(tmpDir, "query")
	kStrs := make([]string, len(kmerSizes))
	for i, k := range kmerSizes {
		kStrs[i] = strconv.Itoa(k)
	}
	args := []string{"sketch", "-o", prefix, "--k-vals", strings.Join(kStrs, ",")}
	if threads > 0 {
		args = append(args, "--threads", strconv.Itoa(threads))
	}
	args = append(args, inputs...)
	cmd := exec.Command(bin, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("sketchlib sketch: %w", err)
	}
	return tmpDir, prefix, nil
}

func QueryDist(refPrefix, queryPrefix string, kmer, threads, topN int) ([]Match, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, err
	}
	args := []string{"dist", refPrefix, queryPrefix, "-k", strconv.Itoa(kmer), "--ani"}
	if threads > 0 {
		args = append(args, "--threads", strconv.Itoa(threads))
	}
	cmd := exec.Command(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("sketchlib dist: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("sketchlib dist: %w", err)
	}
	matches, err := ParseDistOutput(strings.NewReader(string(out)))
	if err != nil {
		return nil, err
	}
	if topN > 0 && len(matches) > topN {
		matches = matches[:topN]
	}
	return matches, nil
}
