// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/codec"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/elastic-agent-libs/logp"
)

type client struct {
	log      *logp.Logger
	config   Config
	Conn     clickhouse.Conn
	observer outputs.Observer
	codec    codec.Codec
	index    string
}

func newClient(cfg Config, observer outputs.Observer, codec codec.Codec, index string) (*client, error) {
	c := &client{
		log:      logp.NewLogger("clickhouse"),
		config:   cfg,
		codec:    codec,
		observer: observer,
		index:    index,
	}
	return c, nil
}

func (c *client) Connect() error {

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{c.config.Host},
		Auth: clickhouse.Auth{
			Database: c.config.DbName,
			Username: c.config.UserName,
			Password: c.config.PassWord,
		},
		DialContext: func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", addr)
		},
		Debug: false,
		Debugf: func(format string, v ...interface{}) {
			fmt.Printf(format, v)
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 600,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:      time.Duration(10) * time.Second,
		MaxOpenConns:     5,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Duration(10) * time.Minute,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	})
	if err != nil {
		c.log.Errorw("can not connect clickhouse server", "host", c.config.Host)
		return err
	}
	if err := conn.Ping(context.Background()); err != nil {
		c.log.Errorf("connect clickhouse server failed, err: %v", err)
		return err
	} else {
		c.log.Info("connect clickhouse server successful")
	}

	c.Conn = conn
	return nil
}

func (c *client) Close() error {
	return c.Conn.Close()
}

func (c *client) Publish(_ context.Context, batch publisher.Batch) error {
	st := c.observer
	events := batch.Events()

	batchData, succEventNum := c.getBatchRows(events)
	if succEventNum == 0 {
		batch.Drop()
		c.log.Errorf("batch drop")
		return errors.New("batch filter row failed, batch droped")
	}

	st.NewBatch(len(events))
	filterDroped := len(events) - succEventNum
	if filterDroped > 0 {
		st.Dropped(filterDroped)
	}

	retryEvents := make([]publisher.Event, 0)
	sendDroped := 0
	var lastErr error
	for _, v := range batchData {

		if err := c.sendToTables(v); err != nil {
			c.log.Errorf("send to table err: %v", err)
			lastErr = err

			// dial tcp 10.32.20.146:9000: connect: connection refused
			// dial tcp: lookup clickhouse on 127.0.0.11:53: server misbehaving"
			if strings.Contains(fmt.Sprintf("%s", err), "connection refused") || strings.Contains(fmt.Sprintf("%s", err), "server misbehaving") {
				for _, e_key := range v.EventKeys {
					retryEvents = append(retryEvents, events[e_key])
				}
				c.log.Errorf("connect ck refused, will retry evnet: %d", len(v.EventKeys))
			} else { //other error
				sendDroped += len(v.EventKeys)
			}
		} else {
			c.log.Infof("insert num %d", len(v.EventKeys))
		}
	}

	st.Dropped(sendDroped)
	st.Acked(len(events) - filterDroped - sendDroped)

	if len(retryEvents) > 0 {
		batch.RetryEvents(retryEvents)
		c.log.Errorf("batch retry evnet: %d", len(retryEvents))
	} else {
		batch.ACK()
	}

	return lastErr
}

func (c *client) String() string {
	return "clickhouse"
}

// split table rows
func (c *client) getBatchRows(data []publisher.Event) (batchRows, int) {
	batchs := make(batchRows)
	succEventNum := 0

	for index := range data {
		event := &data[index].Content
		bulkItems := []interface{}{}

		for _, column := range c.config.Columns {
			row, _ := event.GetValue(column)
			bulkItems = append(bulkItems, row)
		}
		succEventNum++
		lineRow := make([][]interface{}, 0)
		eventKeys := make([]int, 0)
		lineRow = append(lineRow, bulkItems)
		eventKeys = append(eventKeys, index)
		batchs[c.config.TableName] = tableData{
			Table:     c.config.TableName,
			Columns:   c.config.Columns,
			Rows:      lineRow,
			EventKeys: eventKeys,
		}

	}
	return batchs, succEventNum
}

func (c *client) sendToTables(v tableData) error {
	tableName := v.Table
	columnStr := strings.Join(v.Columns, ",")
	sql := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES ", c.config.DbName, tableName, columnStr)

	num := 0
	for _, line := range v.Rows {
		valueStr := "("
		for _, column := range line {
			//nested type
			if reflect.TypeOf(column).String() == "[]interface {}" {
				valueStr += "[" + generateQuotaStr(column.([]interface{})) + "],"
			} else {
				valueStr += fmt.Sprintf("'%s',", column)
			}
		}
		sql += strings.TrimRight(valueStr, ",") + "),"
		num++
	}

	sql = strings.TrimRight(sql, ",")
	c.log.Debugf("batch insert num: %d, sql: %s", num, sql)

	return c.Conn.Exec(context.Background(), sql)
}

func generateQuotaStr(data []interface{}) string {
	var str string
	for _, v := range data {
		str += fmt.Sprintf("'%s',", v)
	}
	str = strings.TrimRight(str, ",")
	return str
}
