/*
 * Minio Cloud Storage, (C) 2015-2016 Minio, Inc.
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
	"bytes"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"testing"
)

// Wrapper for calling ListObjects tests for both XL multiple disks and single node setup.
func TestListObjects(t *testing.T) {
	ExecObjectLayerTest(t, testListObjects)
}

// Unit test for ListObjects in general.
func testListObjects(obj ObjectLayer, instanceType string, t TestErrHandler) {

	testBuckets := []string{
		// This bucket is used for testing ListObject operations.
		"test-bucket-list-object",
		// Will not store any objects in this bucket,
		// Its to test ListObjects on an empty bucket.
		"empty-bucket",
	}
	for _, bucket := range testBuckets {
		err := obj.MakeBucket(bucket)
		if err != nil {
			t.Fatalf("%s : %s", instanceType, err.Error())
		}
	}

	var err error
	testObjects := []struct {
		name    string
		content string
	}{
		{"Asia-maps", "asis-maps"},
		{"Asia/India/India-summer-photos-1", "contentstring"},
		{"Asia/India/Karnataka/Bangalore/Koramangala/pics", "contentstring"},
		{"newPrefix0", "newPrefix0"},
		{"newPrefix1", "newPrefix1"},
		{"newzen/zen/recurse/again/again/again/pics", "recurse"},
		{"obj0", "obj0"},
		{"obj1", "obj1"},
		{"obj2", "obj2"},
	}
	for _, object := range testObjects {
		_, err = obj.PutObject(testBuckets[0], object.name, int64(len(object.content)), bytes.NewBufferString(object.content), nil)
		if err != nil {
			t.Fatalf("%s : %s", instanceType, err.Error())
		}

	}

	// Formualting the result data set to be expected from ListObjects call inside the tests,
	// This will be used in testCases and used for asserting the correctness of ListObjects output in the tests.

	resultCases := []ListObjectsInfo{
		// ListObjectsResult-0.
		// Testing for listing all objects in the bucket, (testCase 20,21,22).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
				{Name: "Asia/India/India-summer-photos-1"},
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-1.
		// Used for asserting the truncated case, (testCase 23).
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
				{Name: "Asia/India/India-summer-photos-1"},
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
			},
		},
		// ListObjectsResult-2.
		// (TestCase 24).
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
				{Name: "Asia/India/India-summer-photos-1"},
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
				{Name: "newPrefix0"},
			},
		},
		// ListObjectsResult-3.
		// (TestCase 25).
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
				{Name: "Asia/India/India-summer-photos-1"},
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
			},
		},
		// ListObjectsResult-4.
		// Again used for truncated case.
		// (TestCase 26).
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
			},
		},
		// ListObjectsResult-5.
		// Used for Asserting prefixes.
		// Used for test case with prefix "new", (testCase 27-29).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
			},
		},
		// ListObjectsResult-6.
		// Used for Asserting prefixes.
		// Used for test case with prefix = "obj", (testCase 30).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-7.
		// Used for Asserting prefixes and truncation.
		// Used for test case with prefix = "new" and maxKeys = 1, (testCase 31).
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "newPrefix0"},
			},
		},
		// ListObjectsResult-8.
		// Used for Asserting prefixes.
		// Used for test case with prefix = "obj" and maxKeys = 2, (testCase 32).
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "obj0"},
				{Name: "obj1"},
			},
		},
		// ListObjectsResult-9.
		// Used for asserting the case with marker, but without prefix.
		//marker is set to "newPrefix0" in the testCase, (testCase 33).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-10.
		//marker is set to "newPrefix1" in the testCase, (testCase 34).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-11.
		//marker is set to "obj0" in the testCase, (testCase 35).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-12.
		// Marker is set to "obj1" in the testCase, (testCase 36).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-13.
		// Marker is set to "man" in the testCase, (testCase37).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-14.
		// Marker is set to "Abc" in the testCase, (testCase 39).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
				{Name: "Asia/India/India-summer-photos-1"},
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-15.
		// Marker is set to "Asia/India/India-summer-photos-1" in the testCase, (testCase 40).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-16.
		// Marker is set to "Asia/India/Karnataka/Bangalore/Koramangala/pics" in the testCase, (testCase 41).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-17.
		// Used for asserting the case with marker, without prefix but with truncation.
		// Marker =  "newPrefix0" & maxKeys = 3 in the testCase, (testCase42).
		// Output truncated to 3 values.
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
				{Name: "obj0"},
			},
		},
		// ListObjectsResult-18.
		// Marker = "newPrefix1" & maxkeys = 1 in the testCase, (testCase43).
		// Output truncated to 1 value.
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "newzen/zen/recurse/again/again/again/pics"},
			},
		},
		// ListObjectsResult-19.
		// Marker = "obj0" & maxKeys = 1 in the testCase, (testCase44).
		// Output truncated to 1 value.
		{
			IsTruncated: true,
			Objects: []ObjectInfo{
				{Name: "obj1"},
			},
		},
		// ListObjectsResult-20.
		// Marker = "obj0" & prefix = "obj" in the testCase, (testCase 45).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-21.
		// Marker = "obj1" & prefix = "obj" in the testCase, (testCase 46).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-22.
		// Marker = "newPrefix0" & prefix = "new" in the testCase,, (testCase 47).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix1"},
				{Name: "newzen/zen/recurse/again/again/again/pics"},
			},
		},
		// ListObjectsResult-23.
		// Prefix is set to "Asia/India/" in the testCase, and delimiter is not set (testCase 55).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "Asia/India/India-summer-photos-1"},
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
			},
		},

		// ListObjectsResult-24.
		// Prefix is set to "Asia" in the testCase, and delimiter is not set (testCase 56).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
				{Name: "Asia/India/India-summer-photos-1"},
				{Name: "Asia/India/Karnataka/Bangalore/Koramangala/pics"},
			},
		},

		// ListObjectsResult-25.
		// Prefix is set to "Asia" in the testCase, and delimiter is set (testCase 57).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "Asia-maps"},
			},
		},
		// ListObjectsResult-26.
		// prefix = "new" and delimiter is set in the testCase.(testCase 58).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
			},
		},
		// ListObjectsResult-27.
		// Prefix is set to "Asia/India/" in the testCase, and delimiter is set to forward slash '/' (testCase 59).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "Asia/India/India-summer-photos-1"},
			},
		},
		// ListObjectsResult-28.
		// Marker is set to "Asia/India/India-summer-photos-1" and delimiter set in the testCase, (testCase 60).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-29.
		// Marker is set to "Asia/India/Karnataka/Bangalore/Koramangala/pics" in the testCase and delimeter set, (testCase 61).
		{
			IsTruncated: false,
			Objects: []ObjectInfo{
				{Name: "newPrefix0"},
				{Name: "newPrefix1"},
				{Name: "obj0"},
				{Name: "obj1"},
				{Name: "obj2"},
			},
		},
		// ListObjectsResult-30.
		// Prefix and Delimiter is set to '/', (testCase 62).
		{
			IsTruncated: false,
			Objects:     []ObjectInfo{},
		},
	}

	testCases := []struct {
		// Inputs to ListObjects.
		bucketName string
		prefix     string
		marker     string
		delimeter  string
		maxKeys    int
		// Expected output of ListObjects.
		result ListObjectsInfo
		err    error
		// Flag indicating whether the test is expected to pass or not.
		shouldPass bool
	}{
		// Test cases with invalid bucket names ( Test number 1-4 ).
		{".test", "", "", "", 0, ListObjectsInfo{}, BucketNameInvalid{Bucket: ".test"}, false},
		{"Test", "", "", "", 0, ListObjectsInfo{}, BucketNameInvalid{Bucket: "Test"}, false},
		{"---", "", "", "", 0, ListObjectsInfo{}, BucketNameInvalid{Bucket: "---"}, false},
		{"ad", "", "", "", 0, ListObjectsInfo{}, BucketNameInvalid{Bucket: "ad"}, false},
		// Using an existing file for bucket name, but its not a directory (5).
		{"simple-file.txt", "", "", "", 0, ListObjectsInfo{}, BucketNotFound{Bucket: "simple-file.txt"}, false},
		// Valid bucket names, but they donot exist (6-8).
		{"volatile-bucket-1", "", "", "", 0, ListObjectsInfo{}, BucketNotFound{Bucket: "volatile-bucket-1"}, false},
		{"volatile-bucket-2", "", "", "", 0, ListObjectsInfo{}, BucketNotFound{Bucket: "volatile-bucket-2"}, false},
		{"volatile-bucket-3", "", "", "", 0, ListObjectsInfo{}, BucketNotFound{Bucket: "volatile-bucket-3"}, false},
		// Valid, existing bucket, but sending invalid delimeter values (9-10).
		// Empty string < "" > and forward slash < / > are the ony two valid arguments for delimeter.
		{"test-bucket-list-object", "", "", "*", 0, ListObjectsInfo{}, fmt.Errorf("delimiter '%s' is not supported", "*"), false},
		{"test-bucket-list-object", "", "", "-", 0, ListObjectsInfo{}, fmt.Errorf("delimiter '%s' is not supported", "-"), false},
		// Testing for failure cases with both perfix and marker (11).
		// The prefix and marker combination to be valid it should satisy strings.HasPrefix(marker, prefix).
		{"test-bucket-list-object", "asia", "europe-object", "", 0, ListObjectsInfo{}, fmt.Errorf("Invalid combination of marker '%s' and prefix '%s'", "europe-object", "asia"), false},
		// Setting a non-existing directory to be prefix (12-13).
		{"empty-bucket", "europe/france/", "", "", 1, ListObjectsInfo{}, nil, true},
		{"empty-bucket", "europe/tunisia/", "", "", 1, ListObjectsInfo{}, nil, true},
		// Testing on empty bucket, that is, bucket without any objects in it (14).
		{"empty-bucket", "", "", "", 0, ListObjectsInfo{}, nil, true},
		// Setting maxKeys to negative value (15-16).
		{"empty-bucket", "", "", "", -1, ListObjectsInfo{}, nil, true},
		{"empty-bucket", "", "", "", 1, ListObjectsInfo{}, nil, true},
		// Setting maxKeys to a very large value (17).
		{"empty-bucket", "", "", "", 1111000000000000, ListObjectsInfo{}, nil, true},
		// Testing for all 7 objects in the bucket (18).
		{"test-bucket-list-object", "", "", "", 9, resultCases[0], nil, true},
		//Testing for negative value of maxKey, this should set maxKeys to listObjectsLimit (19).
		{"test-bucket-list-object", "", "", "", -1, resultCases[0], nil, true},
		// Testing for very large value of maxKey, this should set maxKeys to listObjectsLimit (20).
		{"test-bucket-list-object", "", "", "", 1234567891011, resultCases[0], nil, true},
		// Testing for trancated value (21-24).
		{"test-bucket-list-object", "", "", "", 5, resultCases[1], nil, true},
		{"test-bucket-list-object", "", "", "", 4, resultCases[2], nil, true},
		{"test-bucket-list-object", "", "", "", 3, resultCases[3], nil, true},
		{"test-bucket-list-object", "", "", "", 1, resultCases[4], nil, true},
		// Testing with prefix (25-28).
		{"test-bucket-list-object", "new", "", "", 3, resultCases[5], nil, true},
		{"test-bucket-list-object", "new", "", "", 4, resultCases[5], nil, true},
		{"test-bucket-list-object", "new", "", "", 5, resultCases[5], nil, true},
		{"test-bucket-list-object", "obj", "", "", 3, resultCases[6], nil, true},
		// Testing with prefix and truncation (29-30).
		{"test-bucket-list-object", "new", "", "", 1, resultCases[7], nil, true},
		{"test-bucket-list-object", "obj", "", "", 2, resultCases[8], nil, true},
		// Testing with marker, but without prefix and truncation (31-35).
		{"test-bucket-list-object", "", "newPrefix0", "", 5, resultCases[9], nil, true},
		{"test-bucket-list-object", "", "newPrefix1", "", 4, resultCases[10], nil, true},
		{"test-bucket-list-object", "", "obj0", "", 2, resultCases[11], nil, true},
		{"test-bucket-list-object", "", "obj1", "", 1, resultCases[12], nil, true},
		{"test-bucket-list-object", "", "man", "", 10, resultCases[13], nil, true},
		// Marker being set to a value which is greater than and all object names when sorted (36).
		// Expected to send an empty response in this case.
		{"test-bucket-list-object", "", "zen", "", 10, ListObjectsInfo{}, nil, true},
		// Marker being set to a value which is lesser than and all object names when sorted (37).
		// Expected to send all the objects in the bucket in this case.
		{"test-bucket-list-object", "", "Abc", "", 10, resultCases[14], nil, true},
		// Marker is to a hierarhical value (38-39).
		{"test-bucket-list-object", "", "Asia/India/India-summer-photos-1", "", 10, resultCases[15], nil, true},
		{"test-bucket-list-object", "", "Asia/India/Karnataka/Bangalore/Koramangala/pics", "", 10, resultCases[16], nil, true},
		// Testing with marker and truncation, but no prefix (40-42).
		{"test-bucket-list-object", "", "newPrefix0", "", 3, resultCases[17], nil, true},
		{"test-bucket-list-object", "", "newPrefix1", "", 1, resultCases[18], nil, true},
		{"test-bucket-list-object", "", "obj0", "", 1, resultCases[19], nil, true},
		// Testing with both marker and prefix, but without truncation (43-45).
		// The valid combination of marker and prefix should satisfy strings.HasPrefix(marker, prefix).
		{"test-bucket-list-object", "obj", "obj0", "", 2, resultCases[20], nil, true},
		{"test-bucket-list-object", "obj", "obj1", "", 1, resultCases[21], nil, true},
		{"test-bucket-list-object", "new", "newPrefix0", "", 2, resultCases[22], nil, true},
		// Testing with maxKeys set to 0 (46-52).
		// The parameters have to valid.
		{"test-bucket-list-object", "", "obj1", "", 0, ListObjectsInfo{}, nil, true},
		{"test-bucket-list-object", "", "obj0", "", 0, ListObjectsInfo{}, nil, true},
		{"test-bucket-list-object", "new", "", "", 0, ListObjectsInfo{}, nil, true},
		{"test-bucket-list-object", "obj", "", "", 0, ListObjectsInfo{}, nil, true},
		{"test-bucket-list-object", "obj", "obj0", "", 0, ListObjectsInfo{}, nil, true},
		{"test-bucket-list-object", "obj", "obj1", "", 0, ListObjectsInfo{}, nil, true},
		{"test-bucket-list-object", "new", "newPrefix0", "", 0, ListObjectsInfo{}, nil, true},
		// Tests on hierarchical key names as prefix.
		// Without delimteter the code should recurse into the prefix Dir.
		// Tests with prefix, but without delimiter (53-54).
		{"test-bucket-list-object", "Asia/India/", "", "", 10, resultCases[23], nil, true},
		{"test-bucket-list-object", "Asia", "", "", 10, resultCases[24], nil, true},
		// Tests with prefix and delimiter (55-57).
		// With delimeter the code should not recurse into the sub-directories of prefix Dir.
		{"test-bucket-list-object", "Asia", "", "/", 10, resultCases[25], nil, true},
		{"test-bucket-list-object", "new", "", "/", 10, resultCases[26], nil, true},
		{"test-bucket-list-object", "Asia/India/", "", "/", 10, resultCases[27], nil, true},
		// Test with marker set as hierarhical value and with delimiter. (58-59)
		{"test-bucket-list-object", "", "Asia/India/India-summer-photos-1", "/", 10, resultCases[28], nil, true},
		{"test-bucket-list-object", "", "Asia/India/Karnataka/Bangalore/Koramangala/pics", "/", 10, resultCases[29], nil, true},
		// Test with prefix and delimiter set to '/'. (60)
		{"test-bucket-list-object", "/", "", "/", 10, resultCases[30], nil, true},
	}

	for i, testCase := range testCases {
		result, err := obj.ListObjects(testCase.bucketName, testCase.prefix, testCase.marker, testCase.delimeter, testCase.maxKeys)
		if err != nil && testCase.shouldPass {
			t.Errorf("Test %d: %s:  Expected to pass, but failed with: <ERROR> %s", i+1, instanceType, err.Error())
		}
		if err == nil && !testCase.shouldPass {
			t.Errorf("Test %d: %s: Expected to fail with <ERROR> \"%s\", but passed instead", i+1, instanceType, testCase.err.Error())
		}
		// Failed as expected, but does it fail for the expected reason.
		if err != nil && !testCase.shouldPass {
			if !strings.Contains(err.Error(), testCase.err.Error()) {
				t.Errorf("Test %d: %s: Expected to fail with error \"%s\", but instead failed with error \"%s\" instead", i+1, instanceType, testCase.err.Error(), err.Error())
			}
		}
		// Since there are cases for which ListObjects fails, this is
		// necessary. Test passes as expected, but the output values
		// are verified for correctness here.
		if err == nil && testCase.shouldPass {
			// The length of the expected ListObjectsResult.Objects
			// should match in both expected result from test cases
			// and in the output. On failure calling t.Fatalf,
			// otherwise it may lead to index out of range error in
			// assertion following this.
			if len(testCase.result.Objects) != len(result.Objects) {
				t.Fatalf("Test %d: %s: Expected number of object in the result to be '%d', but found '%d' objects instead", i+1, instanceType, len(testCase.result.Objects), len(result.Objects))
			}
			for j := 0; j < len(testCase.result.Objects); j++ {
				if testCase.result.Objects[j].Name != result.Objects[j].Name {
					t.Errorf("Test %d: %s: Expected object name to be \"%s\", but found \"%s\" instead", i+1, instanceType, testCase.result.Objects[j].Name, result.Objects[j].Name)
				}
			}
			if testCase.result.IsTruncated != result.IsTruncated {
				t.Errorf("Test %d: %s: Expected IsTruncated flag to be %v, but instead found it to be %v", i+1, instanceType, testCase.result.IsTruncated, result.IsTruncated)
			}

		}
		// Take ListObject treeWalk go-routine to completion, if available in the treewalk pool.
		if result.IsTruncated {
			_, err = obj.ListObjects(testCase.bucketName, testCase.prefix, result.NextMarker, testCase.delimeter, 1000)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func BenchmarkListObjects(b *testing.B) {
	// Make a temporary directory to use as the obj.
	directory, err := ioutil.TempDir("", "minio-list-benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer removeAll(directory)

	// Create the obj.
	obj, err := newFSObjects(directory)
	if err != nil {
		b.Fatal(err)
	}

	// Create a bucket.
	err = obj.MakeBucket("ls-benchmark-bucket")
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < 20000; i++ {
		key := "obj" + strconv.Itoa(i)
		_, err = obj.PutObject("ls-benchmark-bucket", key, int64(len(key)), bytes.NewBufferString(key), nil)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	// List the buckets over and over and over.
	for i := 0; i < b.N; i++ {
		_, err = obj.ListObjects("ls-benchmark-bucket", "", "obj9000", "", -1)
		if err != nil {
			b.Fatal(err)
		}
	}
}
