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
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"testing"
)

func TestRepeatPutObjectPart(t *testing.T) {
	var objLayer ObjectLayer
	var disks []string
	var err error

	objLayer, disks, err = prepareXL()
	if err != nil {
		t.Fatal(err)
	}

	// cleaning up of temporary test directories
	defer removeRoots(disks)

	err = objLayer.MakeBucket("bucket1")
	if err != nil {
		t.Fatal(err)
	}

	uploadID, err := objLayer.NewMultipartUpload("bucket1", "mpartObj1", nil)
	if err != nil {
		t.Fatal(err)
	}
	fiveMBBytes := bytes.Repeat([]byte("a"), 5*1024*1024)
	md5Writer := md5.New()
	md5Writer.Write(fiveMBBytes)
	md5Hex := hex.EncodeToString(md5Writer.Sum(nil))
	_, err = objLayer.PutObjectPart("bucket1", "mpartObj1", uploadID, 1, 5*1024*1024, bytes.NewReader(fiveMBBytes), md5Hex)
	if err != nil {
		t.Fatal(err)
	}
	// PutObjectPart should succeed even if part already exists. ref: https://github.com/minio/minio/issues/1930
	_, err = objLayer.PutObjectPart("bucket1", "mpartObj1", uploadID, 1, 5*1024*1024, bytes.NewReader(fiveMBBytes), md5Hex)
	if err != nil {
		t.Fatal(err)
	}

}

func TestXLDeleteObjectBasic(t *testing.T) {
	testCases := []struct {
		bucket      string
		object      string
		expectedErr error
	}{
		{".test", "obj", BucketNameInvalid{Bucket: ".test"}},
		{"----", "obj", BucketNameInvalid{Bucket: "----"}},
		{"bucket", "", ObjectNameInvalid{Bucket: "bucket", Object: ""}},
		{"bucket", "obj/", ObjectNameInvalid{Bucket: "bucket", Object: "obj/"}},
		{"bucket", "/obj", ObjectNameInvalid{Bucket: "bucket", Object: "/obj"}},
		{"bucket", "doesnotexist", ObjectNotFound{Bucket: "bucket", Object: "doesnotexist"}},
		{"bucket", "obj", nil},
	}

	// Create an instance of xl backend
	xl, fsDirs, err := prepareXL()
	if err != nil {
		t.Fatal(err)
	}

	// Make bucket for Test 7 to pass
	err = xl.MakeBucket("bucket")
	if err != nil {
		t.Fatal(err)
	}

	// Create object "obj" under bucket "bucket" for Test 7 to pass
	_, err = xl.PutObject("bucket", "obj", int64(len("abcd")), bytes.NewReader([]byte("abcd")), nil)
	if err != nil {
		t.Fatalf("XL Object upload failed: <ERROR> %s", err)
	}
	for i, test := range testCases {
		actualErr := xl.DeleteObject(test.bucket, test.object)
		if test.expectedErr != nil && actualErr != test.expectedErr {
			t.Errorf("Test %d: Expected to fail with %s, but failed with %s", i+1, test.expectedErr, actualErr)
		}
		if test.expectedErr == nil && actualErr != nil {
			t.Errorf("Test %d: Expected to pass, but failed with %s", i+1, actualErr)
		}
	}
	// Cleanup backend directories
	removeRoots(fsDirs)
}

func TestXLDeleteObjectDiskNotFound(t *testing.T) {
	// Create an instance of xl backend.
	obj, fsDirs, err := prepareXL()
	if err != nil {
		t.Fatal(err)
	}

	xl := obj.(xlObjects)

	// Create "bucket"
	err = obj.MakeBucket("bucket")
	if err != nil {
		t.Fatal(err)
	}

	bucket := "bucket"
	object := "object"
	// Create object "obj" under bucket "bucket".
	_, err = obj.PutObject(bucket, object, int64(len("abcd")), bytes.NewReader([]byte("abcd")), nil)

	// for a 16 disk setup, quorum is 9. To simulate disks not found yet
	// quorum is available, we remove disks leaving quorum disks behind.
	for i := range xl.storageDisks[:7] {
		xl.storageDisks[i] = newFaultyDisk(xl.storageDisks[i].(*posix), 0)
	}
	err = obj.DeleteObject(bucket, object)
	if err != nil {
		t.Fatal(err)
	}

	// Create "obj" under "bucket".
	_, err = obj.PutObject(bucket, object, int64(len("abcd")), bytes.NewReader([]byte("abcd")), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Remove one more disk to 'lose' quorum, by setting it to nil.
	xl.storageDisks[7] = &faultyDisk{}
	xl.storageDisks[8] = &faultyDisk{}
	err = obj.DeleteObject(bucket, object)
	if err != toObjectErr(errXLWriteQuorum, bucket, object) {
		t.Errorf("Expected deleteObject to fail with %v, but failed with %v", toObjectErr(errXLWriteQuorum, bucket, object), err)
	}
	// Cleanup backend directories
	removeRoots(fsDirs)
}

func TestGetObjectNoQuorum(t *testing.T) {
	// Create an instance of xl backend.
	obj, fsDirs, err := prepareXL()
	if err != nil {
		t.Fatal(err)
	}

	xl := obj.(xlObjects)

	// Create "bucket"
	err = obj.MakeBucket("bucket")
	if err != nil {
		t.Fatal(err)
	}

	bucket := "bucket"
	object := "object"
	// Create "object" under "bucket".
	_, err = obj.PutObject(bucket, object, int64(len("abcd")), bytes.NewReader([]byte("abcd")), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Disable caching to avoid returning early and not covering other code-paths
	xl.objCacheEnabled = false
	// Make 9 disks offline, which leaves less than quorum number of disks
	// in a 16 disk XL setup. The original disks are 'replaced' with
	// faultyDisks that fail after 'f' successful StorageAPI method
	// invocations, where f - [0,2)
	for f := 0; f < 2; f++ {
		for i := range xl.storageDisks[:9] {
			switch diskType := xl.storageDisks[i].(type) {
			case *posix:
				xl.storageDisks[i] = newFaultyDisk(diskType, f)
			case *faultyDisk:
				xl.storageDisks[i] = newFaultyDisk(diskType.disk, f)
			}
		}
		// Fetch object from store.
		err = xl.GetObject(bucket, object, 0, int64(len("abcd")), ioutil.Discard)
		if err != toObjectErr(errXLReadQuorum, bucket, object) {
			t.Errorf("Expected putObject to fail with %v, but failed with %v", toObjectErr(errXLWriteQuorum, bucket, object), err)
		}
	}
	// Cleanup backend directories.
	removeRoots(fsDirs)
}

func TestPutObjectNoQuorum(t *testing.T) {
	// Create an instance of xl backend.
	obj, fsDirs, err := prepareXL()
	if err != nil {
		t.Fatal(err)
	}

	xl := obj.(xlObjects)

	// Create "bucket"
	err = obj.MakeBucket("bucket")
	if err != nil {
		t.Fatal(err)
	}

	bucket := "bucket"
	object := "object"
	// Create "object" under "bucket".
	_, err = obj.PutObject(bucket, object, int64(len("abcd")), bytes.NewReader([]byte("abcd")), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Make 9 disks offline, which leaves less than quorum number of disks
	// in a 16 disk XL setup. The original disks are 'replaced' with
	// faultyDisks that fail after 'f' successful StorageAPI method
	// invocations, where f - [0,3)
	for f := 0; f < 3; f++ {
		for i := range xl.storageDisks[:9] {
			switch diskType := xl.storageDisks[i].(type) {
			case *posix:
				xl.storageDisks[i] = newFaultyDisk(diskType, f)
			case *faultyDisk:
				xl.storageDisks[i] = newFaultyDisk(diskType.disk, f)
			}
		}
		// Upload new content to same object "object"
		_, err = obj.PutObject(bucket, object, int64(len("abcd")), bytes.NewReader([]byte("abcd")), nil)
		if err != toObjectErr(errXLWriteQuorum, bucket, object) {
			t.Errorf("Expected putObject to fail with %v, but failed with %v", toObjectErr(errXLWriteQuorum, bucket, object), err)
		}
	}
	// Cleanup backend directories.
	removeRoots(fsDirs)
}
