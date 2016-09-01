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

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/cli"
)

var srvConfig serverCmdConfig
var serverCmd = cli.Command{
	Name:  "server",
	Usage: "Start object storage server.",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "address",
			Value: ":9000",
			Usage: "Specify custom server \"ADDRESS:PORT\", defaults to \":9000\".",
		},
		cli.StringFlag{
			Name:  "ignore-disks",
			Usage: "Specify comma separated list of disks that are offline.",
		},
	},
	Action: serverMain,
	CustomHelpTemplate: `NAME:
  minio {{.Name}} - {{.Usage}}

USAGE:
  minio {{.Name}} [FLAGS] PATH [PATH...]

FLAGS:
  {{range .Flags}}{{.}}
  {{end}}
ENVIRONMENT VARIABLES:
  ACCESS:
     MINIO_ACCESS_KEY: Access key string of 5 to 20 characters in length.
     MINIO_SECRET_KEY: Secret key string of 8 to 40 characters in length.

  CACHING:
     MINIO_CACHE_SIZE: Set total cache size in NN[GB|MB|KB]. Defaults to 8GB.
     MINIO_CACHE_EXPIRY: Set cache expiration duration in NN[h|m|s]. Defaults to 72 hours.

EXAMPLES:
  1. Start minio server.
      $ minio {{.Name}} /home/shared

  2. Start minio server bound to a specific IP:PORT, when you have multiple network interfaces.
      $ minio {{.Name}} --address 192.168.1.101:9000 /home/shared

  3. Start minio server on Windows.
      $ minio {{.Name}} C:\MyShare

  4. Start minio server on 12 disks to enable erasure coded layer with 6 data and 6 parity.
      $ minio {{.Name}} /mnt/export1/backend /mnt/export2/backend /mnt/export3/backend /mnt/export4/backend \
          /mnt/export5/backend /mnt/export6/backend /mnt/export7/backend /mnt/export8/backend /mnt/export9/backend \
          /mnt/export10/backend /mnt/export11/backend /mnt/export12/backend

  5. Start minio server on 12 disks while ignoring two disks for initialization.
      $ minio {{.Name}} --ignore-disks=/mnt/export1/backend,/mnt/export2/backend /mnt/export1/backend \
          /mnt/export2/backend /mnt/export3/backend /mnt/export4/backend /mnt/export5/backend /mnt/export6/backend \
          /mnt/export7/backend /mnt/export8/backend /mnt/export9/backend /mnt/export10/backend /mnt/export11/backend \
          /mnt/export12/backend

  6. Start minio server with 4 remote disks on all remote nodes. You need to execute the same command on all nodes
     192.168.1.11, 192.168.1.12, 192.168.1.13 and 192.168.1.14.
      $ export MINIO_ACCESS_KEY=minio
      $ export MINIO_SECRET_KEY=miniostorage
      $ minio {{.Name}} 192.168.1.11:/mnt/export1/backend 192.168.1.12:/mnt/export1/backend \
          192.168.1.13:/mnt/export1/backend 192.168.1.14:/mnt/export1/backend
`,
}

type serverCmdConfig struct {
	serverAddr   string
	disks        []string
	ignoredDisks []string
}

// getListenIPs - gets all the ips to listen on.
func getListenIPs(httpServerConf *http.Server) (hosts []string, port string) {
	host, port, err := net.SplitHostPort(httpServerConf.Addr)
	fatalIf(err, "Unable to parse host address.", httpServerConf.Addr)

	if host != "" {
		hosts = append(hosts, host)
		return hosts, port
	}
	addrs, err := net.InterfaceAddrs()
	fatalIf(err, "Unable to determine network interface address.")
	for _, addr := range addrs {
		if addr.Network() == "ip+net" {
			host := strings.Split(addr.String(), "/")[0]
			if ip := net.ParseIP(host); ip.To4() != nil {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts, port
}

// Finalizes the endpoints based on the host list and port.
func finalizeEndpoints(tls bool, apiServer *http.Server) (endPoints []string) {
	// Get list of listen ips and port.
	hosts, port := getListenIPs(apiServer)

	// Verify current scheme.
	scheme := "http"
	if tls {
		scheme = "https"
	}

	ips := getIPsFromHosts(hosts)

	// Construct proper endpoints.
	for _, ip := range ips {
		endPoints = append(endPoints, fmt.Sprintf("%s://%s:%s", scheme, ip.String(), port))
	}

	// Success.
	return endPoints
}

// initServerConfig initialize server config.
func initServerConfig(c *cli.Context) {
	// Create certs path.
	err := createCertsPath()
	fatalIf(err, "Unable to create \"certs\" directory.")

	// Fetch max conn limit from environment variable.
	if maxConnStr := os.Getenv("MINIO_MAXCONN"); maxConnStr != "" {
		// We need to parse to its integer value.
		globalMaxConn, err = strconv.Atoi(maxConnStr)
		fatalIf(err, "Unable to convert MINIO_MAXCONN=%s environment variable into its integer value.", maxConnStr)
	}

	// Fetch max cache size from environment variable.
	if maxCacheSizeStr := os.Getenv("MINIO_CACHE_SIZE"); maxCacheSizeStr != "" {
		// We need to parse cache size to its integer value.
		globalMaxCacheSize, err = strconvBytes(maxCacheSizeStr)
		fatalIf(err, "Unable to convert MINIO_CACHE_SIZE=%s environment variable into its integer value.", maxCacheSizeStr)
	}

	// Fetch cache expiry from environment variable.
	if cacheExpiryStr := os.Getenv("MINIO_CACHE_EXPIRY"); cacheExpiryStr != "" {
		// We need to parse cache expiry to its time.Duration value.
		globalCacheExpiry, err = time.ParseDuration(cacheExpiryStr)
		fatalIf(err, "Unable to convert MINIO_CACHE_EXPIRY=%s environment variable into its time.Duration value.", cacheExpiryStr)
	}

	// Fetch access keys from environment variables if any and update the config.
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")

	// Validate if both keys are specified and they are valid save them.
	if accessKey != "" && secretKey != "" {
		if !isValidAccessKey.MatchString(accessKey) {
			fatalIf(errInvalidArgument, "Invalid access key.")
		}
		if !isValidSecretKey.MatchString(secretKey) {
			fatalIf(errInvalidArgument, "Invalid secret key.")
		}
		// Set new credentials.
		serverConfig.SetCredential(credential{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
		})
		// Save new config.
		err = serverConfig.Save()
		fatalIf(err, "Unable to save config.")
	}

	// Set maxOpenFiles, This is necessary since default operating
	// system limits of 1024, 2048 are not enough for Minio server.
	setMaxOpenFiles()
	// Set maxMemory, This is necessary since default operating
	// system limits might be changed and we need to make sure we
	// do not crash the server so the set the maxCacheSize appropriately.
	setMaxMemory()

	// Do not fail if this is not allowed, lower limits are fine as well.
}

// Validate if input disks are sufficient for initializing XL.
func checkSufficientDisks(disks []string) error {
	// Verify total number of disks.
	totalDisks := len(disks)
	if totalDisks > maxErasureBlocks {
		return errXLMaxDisks
	}
	if totalDisks < minErasureBlocks {
		return errXLMinDisks
	}

	// isEven function to verify if a given number if even.
	isEven := func(number int) bool {
		return number%2 == 0
	}

	// Verify if we have even number of disks.
	// only combination of 4, 6, 8, 10, 12, 14, 16 are supported.
	if !isEven(totalDisks) {
		return errXLNumDisks
	}

	// Success.
	return nil
}

// Validates if disks are of supported format, invalid arguments are rejected.
func checkNamingDisks(disks []string) error {
	for _, disk := range disks {
		_, _, err := splitNetPath(disk)
		if err != nil {
			return err
		}
	}
	return nil
}

// Check server arguments.
func checkServerSyntax(c *cli.Context) {
	if !c.Args().Present() || c.Args().First() == "help" {
		cli.ShowCommandHelpAndExit(c, "server", 1)
	}
	disks := c.Args()
	if len(disks) > 1 {
		// Validate if input disks have duplicates in them.
		err := checkDuplicates(disks)
		fatalIf(err, "Invalid disk arguments for server.")

		// Validate if input disks are sufficient for erasure coded setup.
		err = checkSufficientDisks(disks)
		fatalIf(err, "Invalid disk arguments for server.")

		// Validate if input disks are properly named in accordance with either
		//  - /mnt/disk1
		//  - ip:/mnt/disk1
		err = checkNamingDisks(disks)
		fatalIf(err, "Invalid disk arguments for server.")
	}
}

// Extract port number from address address should be of the form host:port.
func getPort(address string) int {
	_, portStr, _ := net.SplitHostPort(address)

	// If port empty, default to port '80'
	if portStr == "" {
		portStr = "80"
		// if SSL is enabled, choose port as "443" instead.
		if isSSL() {
			portStr = "443"
		}
	}

	// Return converted port number.
	portInt, err := strconv.Atoi(portStr)
	fatalIf(err, "Invalid port number.")
	return portInt
}

// Returns if slice of disks is a distributed setup.
func isDistributedSetup(disks []string) (isDist bool) {
	// Port to connect to for the lock servers in a distributed setup.
	for _, disk := range disks {
		if !isLocalStorage(disk) {
			// One or more disks supplied as arguments are not
			// attached to the local node.
			isDist = true
		}
	}
	return isDist
}

// Format disks before initialization object layer.
func formatDisks(disks, ignoredDisks []string) error {
	storageDisks, err := waitForFormattingDisks(disks, ignoredDisks)
	for i := range storageDisks {
		switch storage := storageDisks[i].(type) {
		// Closing associated TCP connections since
		// []StorageAPI is garbage collected eventually.
		case networkStorage:
			storage.rpcClient.Close()
		}
	}
	if err != nil {
		return err
	}
	if isLocalStorage(disks[0]) {
		// notify every one else that they can try init again.
		for i := range storageDisks {
			switch storage := storageDisks[i].(type) {
			// Closing associated TCP connections since
			// []StorageAPI is garage collected eventually.
			case networkStorage:
				var reply GenericReply
				_ = storage.rpcClient.Call("Storage.TryInitHandler", &GenericArgs{}, &reply)
			}
		}
	}
	return nil
}

// serverMain handler called for 'minio server' command.
func serverMain(c *cli.Context) {
	// Check 'server' cli arguments.
	checkServerSyntax(c)

	// Initialize server config.
	initServerConfig(c)

	// If https.
	tls := isSSL()

	// Server address.
	serverAddress := c.String("address")

	// Check if requested port is available.
	port := getPort(serverAddress)
	err := checkPortAvailability(port)
	fatalIf(err, "Port unavailable %d", port)

	// Disks to be ignored in server init, to skip format healing.
	ignoredDisks := strings.Split(c.String("ignore-disks"), ",")

	// Disks to be used in server init.
	disks := c.Args()

	isDist := isDistributedSetup(disks)
	// Set nodes for dsync for distributed setup.
	if isDist {
		err = initDsyncNodes(disks, port)
		fatalIf(err, "Unable to initialize distributed locking")
	}

	// Initialize name space lock.
	initNSLock(isDist)

	// Configure server.
	srvConfig = serverCmdConfig{
		serverAddr:   serverAddress,
		disks:        disks,
		ignoredDisks: ignoredDisks,
	}

	// Initialize and monitor shutdown signals.
	err = initGracefulShutdown(os.Exit)
	fatalIf(err, "Unable to initialize graceful shutdown operation")

	// Configure server.
	handler := configureServerHandler(srvConfig)

	apiServer := NewMuxServer(serverAddress, handler)

	// Fetch endpoints which we are going to serve from.
	endPoints := finalizeEndpoints(tls, &apiServer.Server)

	// Prints the formatted startup message.
	printStartupMessage(endPoints)

	// Register generic callbacks.
	globalShutdownCBs.AddGenericCB(func() errCode {
		// apiServer.Stop()
		return exitSuccess
	})

	// Start server.
	// Configure TLS if certs are available.
	wait := make(chan struct{}, 1)
	go func(tls bool, wait chan<- struct{}) {
		if tls {
			err = apiServer.ListenAndServeTLS(mustGetCertFile(), mustGetKeyFile())
		} else {
			// Fallback to http.
			err = apiServer.ListenAndServe()
		}
		wait <- struct{}{}

	}(tls, wait)
	err = formatDisks(disks, ignoredDisks)
	if err != nil {
		// FIXME: call graceful exit
		errorIf(err, "formatting storage disks failed")
		return
	}
	newObject, err := newObjectLayer(disks, ignoredDisks)
	if err != nil {
		// FIXME: call graceful exit
		errorIf(err, "intializing object layer failed")
		return
	}
	objLayerMutex.Lock()
	globalObjectAPI = newObject
	objLayerMutex.Unlock()
	<-wait

	fatalIf(err, "Failed to start minio server.")
}
