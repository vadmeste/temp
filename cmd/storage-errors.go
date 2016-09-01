/*
 * Minio Cloud Storage, (C) 2015, 2016 Minio, Inc.
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

import "errors"

// errUnexpected - unexpected error, requires manual intervention.
var errUnexpected = errors.New("Unexpected error, please report this issue at https://github.com/minio/minio/issues")

// errCorruptedFormat - corrupted backend format.
var errCorruptedFormat = errors.New("corrupted backend format")

// errUnformattedDisk - unformatted disk found.
var errUnformattedDisk = errors.New("unformatted disk found")

// errDiskFull - cannot create volume or files when disk is full.
var errDiskFull = errors.New("disk path full")

// errDiskNotFount - cannot find the underlying configured disk anymore.
var errDiskNotFound = errors.New("disk not found")

// errDiskAccessDenied - we don't have write permissions on disk.
var errDiskAccessDenied = errors.New("disk access denied")

// errFileNotFound - cannot find the file.
var errFileNotFound = errors.New("file not found")

// errFileNameTooLong - given file name is too long than supported length.
var errFileNameTooLong = errors.New("file name too long")

// errVolumeExists - cannot create same volume again.
var errVolumeExists = errors.New("volume already exists")

// errIsNotRegular - not of regular file type.
var errIsNotRegular = errors.New("not of regular file type")

// errVolumeNotFound - cannot find the volume.
var errVolumeNotFound = errors.New("volume not found")

// errVolumeNotEmpty - volume not empty.
var errVolumeNotEmpty = errors.New("volume is not empty")

// errVolumeAccessDenied - cannot access volume, insufficient permissions.
var errVolumeAccessDenied = errors.New("volume access denied")

// errVolumeAccessDenied - cannot access file, insufficient permissions.
var errFileAccessDenied = errors.New("file access denied")

// errVolumeBusy - remote disk is not connected to yet.
var errVolumeBusy = errors.New("volume is busy")
