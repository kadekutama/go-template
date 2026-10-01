package rbac

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// PolicyLoader loads policies from a backing store with hot reload.
type PolicyLoader interface {
	Load() (policies []PolicyRule, grouping []GroupingRule, err error)
}

// CSVLoader loads the checked-in default policies for tests and bootstrap.
type CSVLoader struct {
	path string
}

// NewCSVLoader builds a CSV policy loader for path.
func NewCSVLoader(path string) *CSVLoader {
	return &CSVLoader{path: path}
}

// Load reads policies.csv (p lines + g lines) from disk.
func (l *CSVLoader) Load() ([]PolicyRule, []GroupingRule, error) {
	if l == nil || l.path == "" {
		return nil, nil, fmt.Errorf("%w: path required", ErrPolicyInvalid)
	}

	file, err := os.Open(l.path)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: open", ErrPolicyInvalid)
	}

	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: parse", ErrPolicyInvalid)
	}

	var policies []PolicyRule

	var grouping []GroupingRule

	for _, row := range rows {
		if len(row) == 0 {
			continue
		}

		kind := strings.TrimSpace(row[0])
		if kind == "p" && len(row) >= 6 {
			policies = append(policies, PolicyRule{
				Sub:    strings.TrimSpace(row[1]),
				Dom:    strings.TrimSpace(row[2]),
				Obj:    strings.TrimSpace(row[3]),
				Act:    strings.TrimSpace(row[4]),
				Effect: strings.TrimSpace(row[5]),
			})
		}

		if kind == "g" && len(row) >= 4 {
			grouping = append(grouping, GroupingRule{
				User: strings.TrimSpace(row[1]),
				Role: strings.TrimSpace(row[2]),
				Dom:  strings.TrimSpace(row[3]),
			})
		}
	}

	return policies, grouping, nil
}
