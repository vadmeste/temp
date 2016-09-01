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
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// Validate all the ListObjects query arguments, returns an APIErrorCode
// if one of the args do not meet the required conditions.
// Special conditions required by Minio server are as below
// - delimiter if set should be equal to '/', otherwise the request is rejected.
// - marker if set should have a common prefix with 'prefix' param, otherwise
//   the request is rejected.
func listObjectsValidateArgs(prefix, marker, delimiter string, maxKeys int) APIErrorCode {
	// Max keys cannot be negative.
	if maxKeys < 0 {
		return ErrInvalidMaxKeys
	}

	/// Minio special conditions for ListObjects.

	// Verify if delimiter is anything other than '/', which we do not support.
	if delimiter != "" && delimiter != "/" {
		return ErrNotImplemented
	}
	// Marker is set validate pre-condition.
	if marker != "" {
		// Marker not common with prefix is not implemented.
		if !strings.HasPrefix(marker, prefix) {
			return ErrNotImplemented
		}
	}
	// Success.
	return ErrNone
}

// ListObjectsV2Handler - GET Bucket (List Objects) Version 2.
// --------------------------
// This implementation of the GET operation returns some or all (up to 1000)
// of the objects in a bucket. You can use the request parameters as selection
// criteria to return a subset of the objects in a bucket.
//
// NOTE: It is recommended that this API to be used for application development.
// Minio continues to support ListObjectsV1 for supporting legacy tools.
func (api objectAPIHandlers) ListObjectsV2Handler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucket := vars["bucket"]

	objectAPI := api.ObjectAPI()
	if objectAPI == nil {
		writeErrorResponse(w, r, ErrServerNotInitialized, r.URL.Path)
		return
	}

	switch getRequestAuthType(r) {
	default:
		// For all unknown auth types return error.
		writeErrorResponse(w, r, ErrAccessDenied, r.URL.Path)
		return
	case authTypeAnonymous:
		// http://docs.aws.amazon.com/AmazonS3/latest/dev/using-with-s3-actions.html
		if s3Error := enforceBucketPolicy(bucket, "s3:ListBucket", r.URL); s3Error != ErrNone {
			writeErrorResponse(w, r, s3Error, r.URL.Path)
			return
		}
	case authTypeSigned, authTypePresigned:
		if s3Error := isReqAuthenticated(r); s3Error != ErrNone {
			writeErrorResponse(w, r, s3Error, r.URL.Path)
			return
		}
	}
	// Extract all the listObjectsV2 query params to their native values.
	prefix, token, startAfter, delimiter, maxKeys, _ := getListObjectsV2Args(r.URL.Query())

	// In ListObjectsV2 'continuation-token' is the marker.
	marker := token
	// Check if 'continuation-token' is empty.
	if token == "" {
		// Then we need to use 'start-after' as marker instead.
		marker = startAfter
	}
	// Validate all the query params before beginning to serve the request.
	if s3Error := listObjectsValidateArgs(prefix, marker, delimiter, maxKeys); s3Error != ErrNone {
		writeErrorResponse(w, r, s3Error, r.URL.Path)
		return
	}
	// Inititate a list objects operation based on the input params.
	// On success would return back ListObjectsInfo object to be
	// marshalled into S3 compatible XML header.
	listObjectsInfo, err := objectAPI.ListObjects(bucket, prefix, marker, delimiter, maxKeys)
	if err != nil {
		errorIf(err, "Unable to list objects.")
		writeErrorResponse(w, r, toAPIErrorCode(err), r.URL.Path)
		return
	}

	response := generateListObjectsV2Response(bucket, prefix, token, startAfter, delimiter, maxKeys, listObjectsInfo)
	// Write headers
	setCommonHeaders(w)
	// Write success response.
	writeSuccessResponse(w, encodeResponse(response))
}

// ListObjectsV1Handler - GET Bucket (List Objects) Version 1.
// --------------------------
// This implementation of the GET operation returns some or all (up to 1000)
// of the objects in a bucket. You can use the request parameters as selection
// criteria to return a subset of the objects in a bucket.
//
func (api objectAPIHandlers) ListObjectsV1Handler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucket := vars["bucket"]

	objectAPI := api.ObjectAPI()
	if objectAPI == nil {
		writeErrorResponse(w, r, ErrServerNotInitialized, r.URL.Path)
		return
	}

	switch getRequestAuthType(r) {
	default:
		// For all unknown auth types return error.
		writeErrorResponse(w, r, ErrAccessDenied, r.URL.Path)
		return
	case authTypeAnonymous:
		// http://docs.aws.amazon.com/AmazonS3/latest/dev/using-with-s3-actions.html
		if s3Error := enforceBucketPolicy(bucket, "s3:ListBucket", r.URL); s3Error != ErrNone {
			writeErrorResponse(w, r, s3Error, r.URL.Path)
			return
		}
	case authTypeSigned, authTypePresigned:
		if s3Error := isReqAuthenticated(r); s3Error != ErrNone {
			writeErrorResponse(w, r, s3Error, r.URL.Path)
			return
		}
	}

	// Extract all the litsObjectsV1 query params to their native values.
	prefix, marker, delimiter, maxKeys, _ := getListObjectsV1Args(r.URL.Query())

	// Validate all the query params before beginning to serve the request.
	if s3Error := listObjectsValidateArgs(prefix, marker, delimiter, maxKeys); s3Error != ErrNone {
		writeErrorResponse(w, r, s3Error, r.URL.Path)
		return
	}

	// Inititate a list objects operation based on the input params.
	// On success would return back ListObjectsInfo object to be
	// marshalled into S3 compatible XML header.
	listObjectsInfo, err := objectAPI.ListObjects(bucket, prefix, marker, delimiter, maxKeys)
	if err != nil {
		errorIf(err, "Unable to list objects.")
		writeErrorResponse(w, r, toAPIErrorCode(err), r.URL.Path)
		return
	}
	response := generateListObjectsV1Response(bucket, prefix, marker, delimiter, maxKeys, listObjectsInfo)
	// Write headers
	setCommonHeaders(w)
	// Write success response.
	writeSuccessResponse(w, encodeResponse(response))
}
