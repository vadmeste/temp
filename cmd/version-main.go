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
	"github.com/minio/cli"
	"github.com/minio/mc/pkg/console"
)

var versionCmd = cli.Command{
	Name:   "version",
	Usage:  "Print version.",
	Action: mainVersion,
	CustomHelpTemplate: `NAME:
   minio {{.Name}} - {{.Usage}}

USAGE:
   minio {{.Name}} {{if .Description}}
`,
}

func mainVersion(ctx *cli.Context) {
	console.Println("Version: " + Version)
	console.Println("Release-Tag: " + ReleaseTag)
	console.Println("Commit-ID: " + CommitID)
}
