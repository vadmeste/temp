/*
 * Minio Cloud Storage, (C) 2015 Minio, Inc.
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
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

// checkDuplicates - function to validate if there are duplicates in a slice of strings.
func checkDuplicates(list []string) error {
	// Empty lists are not allowed.
	if len(list) == 0 {
		return errInvalidArgument
	}
	// Empty keys are not allowed.
	for _, key := range list {
		if key == "" {
			return errInvalidArgument
		}
	}
	listMaps := make(map[string]int)
	// Navigate through each configs and count the entries.
	for _, key := range list {
		listMaps[key]++
	}
	// Validate if there are any duplicate counts.
	for key, count := range listMaps {
		if count != 1 {
			return fmt.Errorf("Duplicate key: \"%s\" found of count: \"%d\"", key, count)
		}
	}
	// No duplicates.
	return nil
}

// splits network path into its components Address and Path.
func splitNetPath(networkPath string) (netAddr, netPath string, err error) {
	if runtime.GOOS == "windows" {
		if volumeName := filepath.VolumeName(networkPath); volumeName != "" {
			return "", networkPath, nil
		}
	}
	networkParts := strings.SplitN(networkPath, ":", 2)
	if len(networkParts) == 1 {
		return "", networkPath, nil
	}
	if networkParts[1] == "" {
		return "", "", &net.AddrError{Err: "Missing path in network path", Addr: networkPath}
	} else if networkParts[0] == "" {
		return "", "", &net.AddrError{Err: "Missing address in network path", Addr: networkPath}
	} else if !filepath.IsAbs(networkParts[1]) {
		return "", "", &net.AddrError{Err: "Network path should be absolute", Addr: networkPath}
	}
	return networkParts[0], networkParts[1], nil
}

// xmlDecoder provide decoded value in xml.
func xmlDecoder(body io.Reader, v interface{}, size int64) error {
	var lbody io.Reader
	if size > 0 {
		lbody = io.LimitReader(body, size)
	} else {
		lbody = body
	}
	d := xml.NewDecoder(lbody)
	return d.Decode(v)
}

// checkValidMD5 - verify if valid md5, returns md5 in bytes.
func checkValidMD5(md5 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(md5))
}

/// http://docs.aws.amazon.com/AmazonS3/latest/dev/UploadingObjects.html
const (
	// maximum object size per PUT request is 5GiB
	maxObjectSize = 1024 * 1024 * 1024 * 5
	// minimum Part size for multipart upload is 5MB
	minPartSize = 1024 * 1024 * 5
	// maximum Part ID for multipart upload is 10000 (Acceptable values range from 1 to 10000 inclusive)
	maxPartID = 10000
)

// isMaxObjectSize - verify if max object size
func isMaxObjectSize(size int64) bool {
	return size > maxObjectSize
}

// Check if part size is more than or equal to minimum allowed size.
func isMinAllowedPartSize(size int64) bool {
	return size >= minPartSize
}

// isMaxPartNumber - Check if part ID is greater than the maximum allowed ID.
func isMaxPartID(partID int) bool {
	return partID > maxPartID
}

func contains(stringList []string, element string) bool {
	for _, e := range stringList {
		if e == element {
			return true
		}
	}
	return false
}

// Represents a type of an exit func which will be invoked upon shutdown signal.
type onExitFunc func(code int)

// Represents a type for all the the callback functions invoked upon shutdown signal.
type cleanupOnExitFunc func() errCode

// Represents a collection of various callbacks executed upon exit signals.
type shutdownCallbacks struct {
	// Protect callbacks list from a concurrent access
	*sync.RWMutex
	// genericCallbacks - is the list of function callbacks executed one by one
	// when a shutdown starts. A callback returns 0 for success and 1 for failure.
	// Failure is considered an emergency error that needs an immediate exit
	genericCallbacks []cleanupOnExitFunc
	// objectLayerCallbacks - contains the list of function callbacks that
	// need to be invoked when a shutdown starts. These callbacks will be called before
	// the general callback shutdowns
	objectLayerCallbacks []cleanupOnExitFunc
}

// globalShutdownCBs stores regular and object storages callbacks
var globalShutdownCBs *shutdownCallbacks

func (s shutdownCallbacks) GetObjectLayerCBs() []cleanupOnExitFunc {
	s.RLock()
	defer s.RUnlock()
	return s.objectLayerCallbacks
}

func (s shutdownCallbacks) GetGenericCBs() []cleanupOnExitFunc {
	s.RLock()
	defer s.RUnlock()
	return s.genericCallbacks
}

func (s *shutdownCallbacks) AddObjectLayerCB(callback cleanupOnExitFunc) error {
	s.Lock()
	defer s.Unlock()
	if callback == nil {
		return errInvalidArgument
	}
	s.objectLayerCallbacks = append(s.objectLayerCallbacks, callback)
	return nil
}

func (s *shutdownCallbacks) AddGenericCB(callback cleanupOnExitFunc) error {
	s.Lock()
	defer s.Unlock()
	if callback == nil {
		return errInvalidArgument
	}
	s.genericCallbacks = append(s.genericCallbacks, callback)
	return nil
}

// Initialize graceful shutdown mechanism.
func initGracefulShutdown(onExitFn onExitFunc) error {
	// Validate exit func.
	if onExitFn == nil {
		return errInvalidArgument
	}
	globalShutdownCBs = &shutdownCallbacks{
		RWMutex: &sync.RWMutex{},
	}
	// Return start monitor shutdown signal.
	return startMonitorShutdownSignal(onExitFn)
}

type shutdownSignal int

const (
	shutdownHalt = iota
	shutdownRestart
)

// Global shutdown signal channel.
var globalShutdownSignalCh = make(chan shutdownSignal, 1)

// Start to monitor shutdownSignal to execute shutdown callbacks
func startMonitorShutdownSignal(onExitFn onExitFunc) error {
	// Validate exit func.
	if onExitFn == nil {
		return errInvalidArgument
	}
	go func() {
		defer close(globalShutdownSignalCh)
		// Monitor signals.
		trapCh := signalTrap(os.Interrupt, syscall.SIGTERM)
		for {
			select {
			case <-trapCh:
				// Initiate graceful shutdown.
				globalShutdownSignalCh <- shutdownHalt
			case signal := <-globalShutdownSignalCh:
				// Call all object storage shutdown callbacks and exit for emergency
				for _, callback := range globalShutdownCBs.GetObjectLayerCBs() {
					exitCode := callback()
					if exitCode != exitSuccess {
						onExitFn(int(exitCode))
					}

				}
				// Call all callbacks and exit for emergency
				for _, callback := range globalShutdownCBs.GetGenericCBs() {
					exitCode := callback()
					if exitCode != exitSuccess {
						onExitFn(int(exitCode))
					}
				}
				// All shutdown callbacks ensure that the server is safely terminated
				// and any concurrent process could be started again
				if signal == shutdownRestart {
					path := os.Args[0]
					cmdArgs := os.Args[1:]
					cmd := exec.Command(path, cmdArgs...)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr

					err := cmd.Start()
					if err != nil {
						errorIf(errors.New("Unable to reboot."), err.Error())
					}
					onExitFn(int(exitSuccess))
				}
				onExitFn(int(exitSuccess))
			}
		}
	}()
	// Successfully started routine.
	return nil
}
