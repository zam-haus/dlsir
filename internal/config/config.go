package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

type ConfigEntry struct {
	Name  string
	Index string
	Value string
}

type ConfigFile struct {
	Name    string
	Entries []ConfigEntry
}

func (conf ConfigFile) GetFilteredEntries(prefix string, include bool) []ConfigEntry {
	res := make([]ConfigEntry, 0)
	for _, entry := range conf.Entries {
		if strings.HasPrefix(entry.Name, prefix) == include {
			res = append(res, entry)
		}
	}

	return res
}

func (conf ConfigFile) GetEntry(name string) (ConfigEntry, error) {
	return getEntry(conf.Entries, name)
}

func entriesFromFile(confFile string) ([]ConfigEntry, error) {
	conf, err := os.ReadFile(confFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %v: %v", confFile, err)
	}

	rx := regexp.MustCompile(`\s*(?P<Key>[^\[\]= \t]+)(\[(?P<Index>\d+)\])?\s*=\s*(?P<Value>.*)\s*`)

	entries := make([]ConfigEntry, 0)

	lines := strings.Split(string(conf), "\n")
	for lineNo, line := range lines {
		line = strings.Trim(line, " \t")
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		m := rx.FindAllStringSubmatch(line, -1)
		if m == nil {
			return nil, fmt.Errorf("line #%v '%v' has invalid format", lineNo, line)
		}

		key, index, value := m[0][1], m[0][3], m[0][4]

		entries = append(entries, ConfigEntry{Name: key, Index: index, Value: value})
	}

	return entries, nil
}

func mergeEntryLists(defaults, specifics []ConfigEntry) []ConfigEntry {
	res := make([]ConfigEntry, len(defaults))

	// don't modify the original array, for reasons...
	copy(res, defaults)
	for _, si := range specifics {
		idx := slices.IndexFunc(res, func(i ConfigEntry) bool { return i.Name == si.Name && i.Index == si.Index })
		if idx != -1 {
			res[idx] = si
		} else {
			res = append(res, si)
		}
	}

	return res
}

func getEntry(entries []ConfigEntry, name string) (ConfigEntry, error) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, nil
		}
	}
	return ConfigEntry{}, errors.New("no such entry")
}

func GetConfigFile(confFile string) (*ConfigFile, error) {
	entries, err := entriesFromFile(confFile)
	if err != nil {
		return nil, err
	}
	return &ConfigFile{Name: confFile, Entries: entries}, nil
}

func GetMergedConfig(specificFile string, defaultFile string) (*ConfigFile, error) {
	phoneEntries, err := entriesFromFile(specificFile)
	if err != nil {
		return nil, err
	}

	defaultEntries, err := entriesFromFile(defaultFile)
	if err != nil {
		return nil, err
	}

	entries := mergeEntryLists(defaultEntries, phoneEntries)
	return &ConfigFile{Name: "MergedConfig("+specificFile+", "+defaultFile+")", Entries: entries}, nil
}

func GetFwItemName(devType string) string {
	return "fw-" + strings.ReplaceAll(strings.ToLower(devType), " ", "")
}
