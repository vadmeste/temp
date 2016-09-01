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
	"io"
	"net/http"
	"testing"
)

// Test all s3 supported auth types.
func TestS3SupportedAuthType(t *testing.T) {
	type testCase struct {
		authT authType
		pass  bool
	}
	// List of all valid and invalid test cases.
	testCases := []testCase{
		// Test 1 - supported s3 type anonymous.
		{
			authT: authTypeAnonymous,
			pass:  true,
		},
		// Test 2 - supported s3 type presigned.
		{
			authT: authTypePresigned,
			pass:  true,
		},
		// Test 3 - supported s3 type signed.
		{
			authT: authTypeSigned,
			pass:  true,
		},
		// Test 4 - supported s3 type with post policy.
		{
			authT: authTypePostPolicy,
			pass:  true,
		},
		// Test 5 - supported s3 type with streaming signed.
		{
			authT: authTypeStreamingSigned,
			pass:  true,
		},
		// Test 6 - JWT is not supported s3 type.
		{
			authT: authTypeJWT,
			pass:  false,
		},
		// Test 7 - unknown auth header is not supported s3 type.
		{
			authT: authTypeUnknown,
			pass:  false,
		},
		// Test 8 - some new auth type is not supported s3 type.
		{
			authT: authType(7),
			pass:  false,
		},
	}
	// Validate all the test cases.
	for i, tt := range testCases {
		ok := isSupportedS3AuthType(tt.authT)
		if ok != tt.pass {
			t.Errorf("Test %d:, Expected %t, got %t", i+1, tt.pass, ok)
		}
	}
}

// TestIsRequestUnsignedPayload - Test validates the Unsigned payload detection logic.
func TestIsRequestUnsignedPayload(t *testing.T) {
	testCases := []struct {
		inputAmzContentHeader string
		expectedResult        bool
	}{
		// Test case - 1.
		// Test case with "X-Amz-Content-Sha256" header set to empty value.
		{"", false},
		// Test case - 2.
		// Test case with "X-Amz-Content-Sha256" header set to  "UNSIGNED-PAYLOAD"
		// The payload is flagged as unsigned When "X-Amz-Content-Sha256" header is set to  "UNSIGNED-PAYLOAD".
		{"UNSIGNED-PAYLOAD", true},
		// Test case - 3.
		// set to a random value.
		{"abcd", false},
	}

	// creating an input HTTP request.
	// Only the headers are relevant for this particular test.
	inputReq, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Error initializing input HTTP request: %v", err)
	}

	for i, testCase := range testCases {
		inputReq.Header.Set("X-Amz-Content-Sha256", testCase.inputAmzContentHeader)
		actualResult := isRequestUnsignedPayload(inputReq)
		if testCase.expectedResult != actualResult {
			t.Errorf("Test %d: Expected the result to `%v`, but instead got `%v`", i+1, testCase.expectedResult, actualResult)
		}
	}
}

// TestIsRequestPresignedSignatureV4 - Test validates the logic for presign signature verision v4 detection.
func TestIsRequestPresignedSignatureV4(t *testing.T) {
	testCases := []struct {
		inputQueryKey   string
		inputQueryValue string
		expectedResult  bool
	}{
		// Test case - 1.
		// Test case with query key ""X-Amz-Credential" set.
		{"", "", false},
		// Test case - 2.
		{"X-Amz-Credential", "", true},
		// Test case - 3.
		{"X-Amz-Content-Sha256", "", false},
	}

	for i, testCase := range testCases {
		// creating an input HTTP request.
		// Only the query parameters are relevant for this particular test.
		inputReq, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			t.Fatalf("Error initializing input HTTP request: %v", err)
		}
		q := inputReq.URL.Query()
		q.Add(testCase.inputQueryKey, testCase.inputQueryValue)
		inputReq.URL.RawQuery = q.Encode()

		actualResult := isRequestPresignedSignatureV4(inputReq)
		if testCase.expectedResult != actualResult {
			t.Errorf("Test %d: Expected the result to `%v`, but instead got `%v`", i+1, testCase.expectedResult, actualResult)
		}
	}
}

// Provides a fully populated http request instance, fails otherwise.
func mustNewRequest(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req, err := newTestRequest(method, urlStr, contentLength, body)
	if err != nil {
		t.Fatalf("Unable to initialize new http request %s", err)
	}
	return req
}

// This is similar to mustNewRequest but additionally the request
// is signed with AWS Signature V4, fails if not able to do so.
func mustNewSignedRequest(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	cred := serverConfig.GetCredential()
	if err := signRequest(req, cred.AccessKeyID, cred.SecretAccessKey); err != nil {
		t.Fatalf("Unable to inititalized new signed http request %s", err)
	}
	return req
}

// Tests is requested authenticated function, tests replies for s3 errors.
func TestIsReqAuthenticated(t *testing.T) {
	path, err := newTestConfig("us-east-1")
	if err != nil {
		t.Fatalf("unable initialize config file, %s", err)
	}
	defer removeAll(path)

	serverConfig.SetCredential(credential{"myuser", "mypassword"})

	// List of test cases for validating http request authentication.
	testCases := []struct {
		req     *http.Request
		s3Error APIErrorCode
	}{
		// When request is nil, internal error is returned.
		{nil, ErrInternalError},
		// When request is unsigned, access denied is returned.
		{mustNewRequest("GET", "http://localhost:9000", 0, nil, t), ErrAccessDenied},
		// When request is properly signed, but has bad Content-MD5 header.
		{mustNewSignedRequest("PUT", "http://localhost:9000", 5, bytes.NewReader([]byte("hello")), t), ErrBadDigest},
		// When request is properly signed, error is none.
		{mustNewSignedRequest("GET", "http://localhost:9000", 0, nil, t), ErrNone},
	}

	// Validates all testcases.
	for _, testCase := range testCases {
		if testCase.s3Error == ErrBadDigest {
			testCase.req.Header.Set("Content-Md5", "garbage")
		}
		if s3Error := isReqAuthenticated(testCase.req); s3Error != testCase.s3Error {
			t.Fatalf("Unexpected s3error returned wanted %d, got %d", testCase.s3Error, s3Error)
		}
	}
}
