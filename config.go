package main

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func itemsFromFile(c *gin.Context, confFile string) []item {
	conf, err := os.ReadFile(confFile)
	if err != nil {
		_log(c, "unable to read file %v: %v", confFile, err)
		return nil
	}

	rx := regexp.MustCompile(`\s*(?P<Key>[^\[\]= \t]+)(\[(?P<Index>\d+)\])?\s*=\s*(?P<Value>.*)\s*`)

	items := make([]item, 0)

	lines := strings.Split(string(conf), "\n")
	for lineNo, line := range lines {
		line = strings.Trim(line, " \t")
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		m := rx.FindAllStringSubmatch(line, -1)
		if m == nil {
			_log(c, "Error: Line %v '%v' has invalid format, skipping...\n", lineNo, line)
			continue
		}

		key, indexStr, value := m[0][1], m[0][3], m[0][4]

		index := 0
		if indexStr != "" {
			index, err = strconv.Atoi(indexStr)
			if err != nil {
				_log(c, "Error: Line %v '%v' has invalid index (this should not happen), skipping...\n", lineNo, line)
				continue
			}
		}

		items = append(items, item{Name: key, Index: index, Value: value})
	}

	return items
}

func mergeItemLists(defaults, specifics []item) []item {
	res := make([]item, len(defaults))

	// don't modify the original array, for reasons...
	copy(res, defaults)
	for _, si := range specifics {
		idx := slices.IndexFunc(res, func(i item) bool { return i.Name == si.Name && i.Index == si.Index })
		if idx != -1 {
			res[idx] = si
		} else {
			res = append(res, si)
		}
	}

	return res
}

func filterItemList(items []item, prefix string, include bool) []item {
	res := make([]item, 0)
	for _, item := range items {
		if strings.HasPrefix(item.Name, prefix) == include {
			res = append(res, item)
		}
	}

	return res
}

func requireItem(items []item, name string) item {
	item, err := getItem(items, name)

	if err != nil {
		_log(nil, "Failed to find required entry %v\n", name)
		os.Exit(1)
		panic("")
	}

	return item
}

func getItem(items []item, name string) (item, error) {
	for _, item := range items {
		if item.Name == name {
			return item, nil
		}
	}
	return item{}, errors.New("no such item")
}

func getPhoneConfig(c *gin.Context, phone *phoneDesc, msg message) []item {
	phoneItems := itemsFromFile(c, confDir+"/"+phone.Mac+".conf")
	defaultItems := itemsFromFile(c, confDir+"/phonedefault.conf")

	return mergeItemLists(defaultItems, phoneItems)
}

func getFwItemName(devType string) string {
	return "fw-" + strings.ReplaceAll(strings.ToLower(devType), " ", "")
}
