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
	"errors"
	"io/ioutil"

	"github.com/Sirupsen/logrus"
	"gopkg.in/olivere/elastic.v3"
)

// elasticQueue is a elasticsearch event notification queue.
type elasticSearchNotify struct {
	Enable bool   `json:"enable"`
	URL    string `json:"url"`
	Index  string `json:"index"`
}

type elasticClient struct {
	*elastic.Client
	params elasticSearchNotify
}

// Connects to elastic search instance at URL.
func dialElastic(esNotify elasticSearchNotify) (*elastic.Client, error) {
	if !esNotify.Enable {
		return nil, errNotifyNotEnabled
	}
	client, err := elastic.NewClient(elastic.SetURL(esNotify.URL), elastic.SetSniff(false))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func newElasticNotify(accountID string) (*logrus.Logger, error) {
	esNotify := serverConfig.GetElasticSearchNotifyByID(accountID)

	// Dial to elastic search.
	client, err := dialElastic(esNotify)
	if err != nil {
		return nil, err
	}

	// Use the IndexExists service to check if a specified index exists.
	exists, err := client.IndexExists(esNotify.Index).Do()
	if err != nil {
		return nil, err
	}
	// Index does not exist, attempt to create it.
	if !exists {
		var createIndex *elastic.IndicesCreateResult
		createIndex, err = client.CreateIndex(esNotify.Index).Do()
		if err != nil {
			return nil, err
		}
		if !createIndex.Acknowledged {
			return nil, errors.New("index not created")
		}
	}

	elasticCl := elasticClient{
		Client: client,
		params: esNotify,
	}

	elasticSearchLog := logrus.New()

	// Disable writing to console.
	elasticSearchLog.Out = ioutil.Discard

	// Add a elasticSearch hook.
	elasticSearchLog.Hooks.Add(elasticCl)

	// Set default JSON formatter.
	elasticSearchLog.Formatter = new(logrus.JSONFormatter)

	// Success, elastic search successfully initialized.
	return elasticSearchLog, nil
}

// Fire is required to implement logrus hook
func (q elasticClient) Fire(entry *logrus.Entry) error {
	_, err := q.Client.Index().Index(q.params.Index).
		Type("event").
		BodyJson(entry.Data).
		Do()

	return err
}

// Required for logrus hook implementation
func (q elasticClient) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.InfoLevel,
	}
}
