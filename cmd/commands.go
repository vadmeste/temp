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

import "github.com/minio/cli"

// Collection of minio commands currently supported are.
var commands = []cli.Command{}

// Collection of minio commands currently supported in a trie tree.
var commandsTree = newTrie()

// Collection of minio flags currently supported.
var globalFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "config-dir, C",
		Value: mustGetConfigPath(),
		Usage: "Path to configuration folder.",
	},
	cli.BoolFlag{
		Name:  "quiet",
		Usage: "Suppress chatty output.",
	},
}

// registerCommand registers a cli command.
func registerCommand(command cli.Command) {
	commands = append(commands, command)
	commandsTree.Insert(command.Name)
}
