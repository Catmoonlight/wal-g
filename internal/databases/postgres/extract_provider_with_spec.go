package postgres

import (
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/wal-g/tracelog"
	"github.com/wal-g/wal-g/internal"
)

const (
	defaultTbspPrefix = "/" + DefaultTablespace + "/"
)

type IncorrectNameError struct {
	error
}

func NewIncorrectNameError(name string) IncorrectNameError {
	return IncorrectNameError{errors.Errorf("Can't make directory by oid or find database in meta with name: '%s'", name)}
}

func (err IncorrectNameError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

type ExtractProviderDBSpec struct {
	ExtractProviderImpl
	onlyDatabases []string
}

func NewExtractProviderDBSpec(onlyDatabases []string) *ExtractProviderDBSpec {
	return &ExtractProviderDBSpec{onlyDatabases: onlyDatabases}
}

func addHardcodedNames(onlyDatabases []string) []string {
	return append(onlyDatabases, []string{
		"template0", "template1", "postgres",
	}...)
}

func (t ExtractProviderDBSpec) Get(
	backup Backup,
	filesToUnwrap map[string]bool,
	skipRedundantTars bool,
	dbDataDir string,
	createNewIncrementalFiles bool,
) (IncrementalTarInterpreter, []internal.ReaderMaker, string, error) {
	_, filesMeta, _ := backup.GetSentinelAndFilesMetadata()

	databases := addHardcodedNames(t.onlyDatabases)
	patterns, err := t.makeRestorePatterns(databases, filesMeta.DatabasesByNames)
	if err != nil {
		return nil, nil, "", err
	}
	err = t.filterFilesToUnwrap(filesToUnwrap, patterns)
	if err != nil {
		return nil, nil, "", err
	}

	return t.ExtractProviderImpl.Get(backup, filesToUnwrap, skipRedundantTars, dbDataDir, createNewIncrementalFiles)
}

func (t ExtractProviderDBSpec) makeRestorePatterns(databases []string, meta DatabasesByNames) ([]string, error) {
	restorePatterns := make([]string, 0)

	for _, key := range databases {
		pattern, err := getPatternByKey(meta, key)
		if err != nil {
			return nil, err
		}
		restorePatterns = append(restorePatterns, pattern)
	}

	return restorePatterns, nil
}

func getPatternByKey(meta DatabasesByNames, key string) (string, error) {
	if data, ok := meta[key]; ok {
		return fmt.Sprintf("/%s/%d/*", DefaultTablespace, data.Oid), nil
	} else {
		return "", NewIncorrectNameError(key)
	}
}

func (t ExtractProviderDBSpec) filterFilesToUnwrap(filesToUnwrap map[string]bool, restorePatterns []string) error {
	for file := range filesToUnwrap {
		if !strings.HasPrefix(file, defaultTbspPrefix) {
			continue
		}

		inPatterns, err := t.isFileInPatterns(restorePatterns, file)
		if err != nil {
			return err
		}
		if !inPatterns {
			delete(filesToUnwrap, file)
		}
	}

	return nil
}

func (t ExtractProviderDBSpec) isFileInPatterns(restorePatterns []string, file string) (bool, error) {
	inPatterns := false
	for _, pattern := range restorePatterns {
		res, err := path.Match(pattern, file)
		if err != nil {
			return false, err
		}
		if res {
			inPatterns = true
			break
		}
	}
	return inPatterns, nil
}
