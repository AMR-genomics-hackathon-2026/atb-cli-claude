package parquet

import (
	"fmt"
	"io"
	"os"

	parquetgo "github.com/parquet-go/parquet-go"
)

// ReadAll reads all rows from a parquet file into a slice of T.
// Column projection is performed based on the struct's parquet tags.
func ReadAll[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening parquet file %q: %w", path, err)
	}
	defer f.Close()

	r := parquetgo.NewGenericReader[T](f)
	defer r.Close()

	rows := make([]T, 0, r.NumRows())
	buf := make([]T, 512)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			rows = append(rows, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading parquet file %q: %w", path, err)
		}
	}

	return rows, nil
}

// ReadFiltered reads rows from a parquet file, keeping only those where fn returns true.
// Column projection is performed based on the struct's parquet tags.
func ReadFiltered[T any](path string, fn func(T) bool) ([]T, error) {
	all, err := ReadAll[T](path)
	if err != nil {
		return nil, err
	}

	result := make([]T, 0)
	for _, row := range all {
		if fn(row) {
			result = append(result, row)
		}
	}

	return result, nil
}
