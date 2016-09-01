/*
 * Minio Cloud Storage, (C) 2016 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Fixed volume name that could be used across tests
const volume = "testvolume"

// Test for delayIsLeafCheck.
func TestDelayIsLeafCheck(t *testing.T) {
	testCases := []struct {
		entries []string
		delay   bool
	}{
		// Test cases where isLeaf check can't be delayed.
		{
			[]string{"a-b/", "a/"},
			false,
		},
		{
			[]string{"a%b/", "a/"},
			false,
		},
		{
			[]string{"a-b-c", "a-b/"},
			false,
		},

		// Test cases where isLeaf check can be delayed.
		{
			[]string{"a-b/", "aa/"},
			true,
		},
		{
			[]string{"a", "a-b"},
			true,
		},
		{
			[]string{"aaa", "bbb"},
			true,
		},
	}
	for i, testCase := range testCases {
		expected := testCase.delay
		got := delayIsLeafCheck(testCase.entries)
		if expected != got {
			t.Errorf("Test %d : Expected %t got %t", i+1, expected, got)
		}
	}
}

// Test for filterMatchingPrefix.
func TestFilterMatchingPrefix(t *testing.T) {
	entries := []string{"a", "aab", "ab", "abbbb", "zzz"}
	testCases := []struct {
		prefixEntry string
		result      []string
	}{
		{
			// Empty prefix should match all entries.
			"",
			[]string{"a", "aab", "ab", "abbbb", "zzz"},
		},
		{
			"a",
			[]string{"a", "aab", "ab", "abbbb"},
		},
		{
			"aa",
			[]string{"aab"},
		},
		{
			// Does not match any of the entries.
			"c",
			[]string{},
		},
	}
	for i, testCase := range testCases {
		expected := testCase.result
		got := filterMatchingPrefix(entries, testCase.prefixEntry)
		if !reflect.DeepEqual(expected, got) {
			t.Errorf("Test %d : expected %v, got %v", i+1, expected, got)
		}
	}
}

// Helper function that creates a volume and files in it.
func createNamespace(disk StorageAPI, volume string, files []string) error {
	// Make a volume.
	err := disk.MakeVol(volume)
	if err != nil {
		return err
	}

	// Create files.
	for _, file := range files {
		err = disk.AppendFile(volume, file, []byte{})
		if err != nil {
			return err
		}
	}
	return err
}

// Test if tree walker returns entries matching prefix alone are received
// when a non empty prefix is supplied.
func testTreeWalkPrefix(t *testing.T, listDir listDirFunc, isLeaf isLeafFunc) {
	// Start the tree walk go-routine.
	prefix := "d/"
	endWalkCh := make(chan struct{})
	twResultCh := startTreeWalk(volume, prefix, "", true, listDir, isLeaf, endWalkCh)

	// Check if all entries received on the channel match the prefix.
	for res := range twResultCh {
		if !strings.HasPrefix(res.entry, prefix) {
			t.Errorf("Entry %s doesn't match prefix %s", res.entry, prefix)
		}
	}
}

// Test if entries received on tree walk's channel appear after the supplied marker.
func testTreeWalkMarker(t *testing.T, listDir listDirFunc, isLeaf isLeafFunc) {
	// Start the tree walk go-routine.
	prefix := ""
	endWalkCh := make(chan struct{})
	twResultCh := startTreeWalk(volume, prefix, "d/g", true, listDir, isLeaf, endWalkCh)

	// Check if only 3 entries, namely d/g/h, i/j/k, lmn are received on the channel.
	expectedCount := 3
	actualCount := 0
	for range twResultCh {
		actualCount++
	}
	if expectedCount != actualCount {
		t.Errorf("Expected %d entries, actual no. of entries were %d", expectedCount, actualCount)
	}
}

// Test tree-walk.
func TestTreeWalk(t *testing.T) {
	fsDir, err := ioutil.TempDir("", "minio-")
	if err != nil {
		t.Errorf("Unable to create tmp directory: %s", err)
	}
	disk, err := newStorageAPI(fsDir)
	if err != nil {
		t.Errorf("Unable to create StorageAPI: %s", err)
	}

	var files = []string{
		"d/e",
		"d/f",
		"d/g/h",
		"i/j/k",
		"lmn",
	}
	err = createNamespace(disk, volume, files)
	if err != nil {
		t.Fatal(err)
	}

	isLeaf := func(volume, prefix string) bool {
		return !strings.HasSuffix(prefix, slashSeparator)
	}
	listDir := listDirFactory(isLeaf, disk)
	// Simple test for prefix based walk.
	testTreeWalkPrefix(t, listDir, isLeaf)
	// Simple test when marker is set.
	testTreeWalkMarker(t, listDir, isLeaf)
	err = removeAll(fsDir)
	if err != nil {
		t.Fatal(err)
	}
}

// Test if tree walk go-routine exits cleanly if tree walk is aborted because of timeout.
func TestTreeWalkTimeout(t *testing.T) {
	fsDir, err := ioutil.TempDir("", "minio-")
	if err != nil {
		t.Errorf("Unable to create tmp directory: %s", err)
	}
	disk, err := newStorageAPI(fsDir)
	if err != nil {
		t.Errorf("Unable to create StorageAPI: %s", err)
	}
	var myfiles []string
	// Create maxObjectsList+1 number of entries.
	for i := 0; i < maxObjectList+1; i++ {
		myfiles = append(myfiles, fmt.Sprintf("file.%d", i))
	}
	err = createNamespace(disk, volume, myfiles)
	if err != nil {
		t.Fatal(err)
	}

	isLeaf := func(volume, prefix string) bool {
		return !strings.HasSuffix(prefix, slashSeparator)
	}
	listDir := listDirFactory(isLeaf, disk)

	// TreeWalk pool with 2 seconds timeout for tree-walk go routines.
	pool := newTreeWalkPool(2 * time.Second)

	endWalkCh := make(chan struct{})
	prefix := ""
	marker := ""
	recursive := true
	resultCh := startTreeWalk(volume, prefix, marker, recursive, listDir, isLeaf, endWalkCh)

	params := listParams{
		bucket:    volume,
		recursive: recursive,
	}
	// Add Treewalk to the pool.
	pool.Set(params, resultCh, endWalkCh)

	// Wait for the Treewalk to timeout.
	<-time.After(3 * time.Second)

	// Read maxObjectList number of entries from the channel.
	// maxObjectsList number of entries would have been filled into the resultCh
	// buffered channel. After the timeout resultCh would get closed and hence the
	// maxObjectsList+1 entry would not be sent in the channel.
	i := 0
	for range resultCh {
		i++
		if i == maxObjectList {
			break
		}
	}

	// The last entry will not be received as the Treewalk goroutine would have exited.
	_, ok := <-resultCh
	if ok {
		t.Error("Tree-walk go routine has not exited after timeout.")
	}
	err = removeAll(fsDir)
	if err != nil {
		t.Error(err)
	}
}

// Test ListDir - listDir should list entries from the first disk, if the first disk is down,
// it should list from the next disk.
func TestListDir(t *testing.T) {
	file1 := "file1"
	file2 := "file2"
	// Create two backend directories fsDir1 and fsDir2.
	fsDir1, err := ioutil.TempDir("", "minio-")
	if err != nil {
		t.Errorf("Unable to create tmp directory: %s", err)
	}
	fsDir2, err := ioutil.TempDir("", "minio-")
	if err != nil {
		t.Errorf("Unable to create tmp directory: %s", err)
	}

	// Create two StorageAPIs disk1 and disk2.
	disk1, err := newStorageAPI(fsDir1)
	if err != nil {
		t.Errorf("Unable to create StorageAPI: %s", err)
	}
	disk2, err := newStorageAPI(fsDir2)
	if err != nil {
		t.Errorf("Unable to create StorageAPI: %s", err)
	}

	// create listDir function.
	listDir := listDirFactory(func(volume, prefix string) bool {
		return !strings.HasSuffix(prefix, slashSeparator)
	}, disk1, disk2)

	// Create file1 in fsDir1 and file2 in fsDir2.
	disks := []StorageAPI{disk1, disk2}
	for i, disk := range disks {
		err = createNamespace(disk, volume, []string{fmt.Sprintf("file%d", i+1)})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Should list "file1" from fsDir1.
	entries, _, err := listDir(volume, "", "")
	if err != nil {
		t.Error(err)
	}
	if len(entries) != 1 {
		t.Fatal("Expected the number of entries to be 1")
	}
	if entries[0] != file1 {
		t.Fatal("Expected the entry to be file1")
	}

	// Remove fsDir1 to test failover.
	err = removeAll(fsDir1)
	if err != nil {
		t.Error(err)
	}

	// Should list "file2" from fsDir2.
	entries, _, err = listDir(volume, "", "")
	if err != nil {
		t.Error(err)
	}
	if len(entries) != 1 {
		t.Fatal("Expected the number of entries to be 1")
	}
	if entries[0] != file2 {
		t.Fatal("Expected the entry to be file2")
	}
	err = removeAll(fsDir2)
	if err != nil {
		t.Error(err)
	}
	// None of the disks are available, should get errDiskNotFound.
	_, _, err = listDir(volume, "", "")
	if err != errDiskNotFound {
		t.Error("expected errDiskNotFound error.")
	}
}

// TestRecursiveWalk - tests if treeWalk returns entries correctly with and
// without recursively traversing prefixes.
func TestRecursiveTreeWalk(t *testing.T) {
	// Create a backend directories fsDir1.
	fsDir1, err := ioutil.TempDir("", "minio-")
	if err != nil {
		t.Errorf("Unable to create tmp directory: %s", err)
	}

	// Create two StorageAPIs disk1.
	disk1, err := newStorageAPI(fsDir1)
	if err != nil {
		t.Errorf("Unable to create StorageAPI: %s", err)
	}

	// Simple isLeaf check, returns true if there is no trailing "/"
	isLeaf := func(volume, prefix string) bool {
		return !strings.HasSuffix(prefix, slashSeparator)
	}

	// Create listDir function.
	listDir := listDirFactory(isLeaf, disk1)

	// Create the namespace.
	var files = []string{
		"d/e",
		"d/f",
		"d/g/h",
		"i/j/k",
		"lmn",
	}
	err = createNamespace(disk1, volume, files)
	if err != nil {
		t.Fatal(err)
	}

	endWalkCh := make(chan struct{})
	testCases := []struct {
		prefix    string
		marker    string
		recursive bool
		expected  map[string]struct{}
	}{
		// with no prefix, no marker and no recursive traversal
		{"", "", false, map[string]struct{}{
			"d/":  {},
			"i/":  {},
			"lmn": {},
		}},
		// with no prefix, no marker and recursive traversal
		{"", "", true, map[string]struct{}{
			"d/f":   {},
			"d/g/h": {},
			"d/e":   {},
			"i/j/k": {},
			"lmn":   {},
		}},
		// with no prefix, marker and no recursive traversal
		{"", "d/e", false, map[string]struct{}{
			"d/f":  {},
			"d/g/": {},
			"i/":   {},
			"lmn":  {},
		}},
		// with no prefix, marker and recursive traversal
		{"", "d/e", true, map[string]struct{}{
			"d/f":   {},
			"d/g/h": {},
			"i/j/k": {},
			"lmn":   {},
		}},
		// with prefix, no marker and no recursive traversal
		{"d/", "", false, map[string]struct{}{
			"d/e":  {},
			"d/f":  {},
			"d/g/": {},
		}},
		// with prefix, no marker and no recursive traversal
		{"d/", "", true, map[string]struct{}{
			"d/e":   {},
			"d/f":   {},
			"d/g/h": {},
		}},
		// with prefix, marker and no recursive traversal
		{"d/", "d/e", false, map[string]struct{}{
			"d/f":  {},
			"d/g/": {},
		}},
		// with prefix, marker and recursive traversal
		{"d/", "d/e", true, map[string]struct{}{
			"d/f":   {},
			"d/g/h": {},
		}},
	}
	for i, testCase := range testCases {
		for entry := range startTreeWalk(volume,
			testCase.prefix, testCase.marker, testCase.recursive,
			listDir, isLeaf, endWalkCh) {
			if _, found := testCase.expected[entry.entry]; !found {
				t.Errorf("Test %d: Expected %s, but couldn't find", i+1, entry.entry)
			}
		}
	}
	err = removeAll(fsDir1)
	if err != nil {
		t.Error(err)
	}
}

func TestSortedness(t *testing.T) {
	// Create a backend directories fsDir1.
	fsDir1, err := ioutil.TempDir("", "minio-")
	if err != nil {
		t.Errorf("Unable to create tmp directory: %s", err)
	}

	// Create two StorageAPIs disk1.
	disk1, err := newStorageAPI(fsDir1)
	if err != nil {
		t.Errorf("Unable to create StorageAPI: %s", err)
	}

	// Simple isLeaf check, returns true if there is no trailing "/"
	isLeaf := func(volume, prefix string) bool {
		return !strings.HasSuffix(prefix, slashSeparator)
	}
	// Create listDir function.
	listDir := listDirFactory(isLeaf, disk1)

	// Create the namespace.
	var files = []string{
		"d/e",
		"d/f",
		"d/g/h",
		"i/j/k",
		"lmn",
	}
	err = createNamespace(disk1, volume, files)
	if err != nil {
		t.Fatal(err)
	}

	endWalkCh := make(chan struct{})
	testCases := []struct {
		prefix    string
		marker    string
		recursive bool
	}{
		// with no prefix, no marker and no recursive traversal
		{"", "", false},
		// with no prefix, no marker and recursive traversal
		{"", "", true},
		// with no prefix, marker and no recursive traversal
		{"", "d/e", false},
		// with no prefix, marker and recursive traversal
		{"", "d/e", true},
		// with prefix, no marker and no recursive traversal
		{"d/", "", false},
		// with prefix, no marker and no recursive traversal
		{"d/", "", true},
		// with prefix, marker and no recursive traversal
		{"d/", "d/e", false},
		// with prefix, marker and recursive traversal
		{"d/", "d/e", true},
	}
	for i, test := range testCases {
		var actualEntries []string
		for entry := range startTreeWalk(volume,
			test.prefix, test.marker, test.recursive,
			listDir, isLeaf, endWalkCh) {
			actualEntries = append(actualEntries, entry.entry)
		}
		if !sort.IsSorted(sort.StringSlice(actualEntries)) {
			t.Error(i+1, "Expected entries to be sort, but it wasn't")
		}
	}

	// Remove directory created for testing
	err = removeAll(fsDir1)
	if err != nil {
		t.Error(err)
	}
}

func TestTreeWalkIsEnd(t *testing.T) {
	// Create a backend directories fsDir1.
	fsDir1, err := ioutil.TempDir("", "minio-")
	if err != nil {
		t.Errorf("Unable to create tmp directory: %s", err)
	}

	// Create two StorageAPIs disk1.
	disk1, err := newStorageAPI(fsDir1)
	if err != nil {
		t.Errorf("Unable to create StorageAPI: %s", err)
	}

	isLeaf := func(volume, prefix string) bool {
		return !strings.HasSuffix(prefix, slashSeparator)
	}
	// Create listDir function.
	listDir := listDirFactory(isLeaf, disk1)

	// Create the namespace.
	var files = []string{
		"d/e",
		"d/f",
		"d/g/h",
		"i/j/k",
		"lmn",
	}
	err = createNamespace(disk1, volume, files)
	if err != nil {
		t.Fatal(err)
	}

	endWalkCh := make(chan struct{})
	testCases := []struct {
		prefix        string
		marker        string
		recursive     bool
		expectedEntry string
	}{
		// with no prefix, no marker and no recursive traversal
		{"", "", false, "lmn"},
		// with no prefix, no marker and recursive traversal
		{"", "", true, "lmn"},
		// with no prefix, marker and no recursive traversal
		{"", "d/e", false, "lmn"},
		// with no prefix, marker and recursive traversal
		{"", "d/e", true, "lmn"},
		// with prefix, no marker and no recursive traversal
		{"d/", "", false, "d/g/"},
		// with prefix, no marker and no recursive traversal
		{"d/", "", true, "d/g/h"},
		// with prefix, marker and no recursive traversal
		{"d/", "d/e", false, "d/g/"},
		// with prefix, marker and recursive traversal
		{"d/", "d/e", true, "d/g/h"},
	}
	for i, test := range testCases {
		var entry treeWalkResult
		for entry = range startTreeWalk(volume, test.prefix, test.marker, test.recursive, listDir, isLeaf, endWalkCh) {
		}
		if entry.entry != test.expectedEntry {
			t.Errorf("Test %d: Expected entry %s, but received %s with the EOF marker", i, test.expectedEntry, entry.entry)
		}
		if !entry.end {
			t.Errorf("Test %d: Last entry %s, doesn't have EOF marker set", i, entry.entry)
		}
	}

	// Remove directory created for testing
	err = removeAll(fsDir1)
	if err != nil {
		t.Error(err)
	}
}
