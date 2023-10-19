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

import "C"
import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/codec"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/elastic-agent-libs/logp"
)

type client struct {
	log         *logp.Logger
	config      Config
	Conn        clickhouse.Conn
	observer    outputs.Observer
	codec       codec.Codec
	index       string
	columnsType map[string]string
}

type ValueWithIndex struct {
	val []interface{}
}
type ClickHouseDescType struct {
	Name              string `db:"name"`
	Type              string `db:"type"`
	DefaultType       string `db:"default_type"`
	DefaultExpression string `db:"default_expression"`
	Comment           string `db:"comment"`
	CodecExpression   string `db:"codec_expression"`
	TTLExpression     string `db:"ttl_expression"`
}

const (
	MAP_TYPE = "Map(String, String)"
)

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

func (c *client) Publish(ctx context.Context, batch publisher.Batch) error {
	st := c.observer
	events := batch.Events()
	c.log.Debugf("本次事件数量: %d", len(events))

	errColumns := c.clickhouseColumns()
	if errColumns != nil {
		c.log.Errorf("get table columns failed", errColumns)
		return errColumns
	}

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
	err := c.batchInsertCk(batchData)
	if err != nil {
		c.log.Errorf("batch size to err: %v", err)
		for index, _ := range events {
			retryEvents = append(retryEvents, events[index])
		}
	}
	st.Dropped(sendDroped)
	st.Acked(len(events) - filterDroped)

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

func (c *client) getBatchRows(data []publisher.Event) ([]map[string]interface{}, int) {
	succEventNum := 0
	var bulkItems []map[string]interface{}
	for index := range data {
		item := make(map[string]interface{})
		event := &data[index].Content
		for _, column := range c.config.Columns {
			value, _ := event.GetValue(column)
			item[column] = value
		}
		bulkItems = append(bulkItems, item)
		succEventNum++
	}
	return bulkItems, succEventNum
}

func (c *client) batchInsertCk(rowsData []map[string]interface{}) error {
	columnStr := strings.Join(c.config.Columns, ",")
	sql := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES ", c.config.DbName, c.config.TableName, columnStr)
	bulk, err := c.Conn.PrepareBatch(context.Background(), sql)
	if err != nil {
		c.log.Errorf("批量预处理sql语句错误,%s", sql)
		return err
	}
	vals, err := c.PrepareData(rowsData)
	if err != nil {
		c.log.Errorf("批量预处理数据错误,", err)
		return err
	}
	fmt.Printf("============>debug", vals)
	for _, val := range vals {
		err = bulk.Append(val[:]...)
		if err != nil {
			c.log.Errorf("batch add data failed", val)
			return err
		}
	}
	c.log.Debugf("batch insert num: %d, sql: %s", len(vals), sql)
	return bulk.Send()
}

func (c *client) PrepareData(batchData []map[string]interface{}) ([][]interface{}, error) {
	var rows [][]interface{}
	for _, data := range batchData {
		result := make([]interface{}, len(c.config.Columns))
		for index, column := range c.config.Columns {
			v, ok := data[column]
			if !ok || v == nil {
				if MAP_TYPE == c.columnsType[column] {
					result[index] = map[string]string{}
				} else {
					result[index] = nil
				}
				continue
			}
			value, err := toClickhouseType(v, c.columnsType[column])
			if err != nil {
				c.log.Errorf("toClickhouseType failed, %s", err)
				return nil, err
			}
			result[index] = value
		}
		rows = append(rows, result)
	}
	return rows, nil
}

func (c *client) clickhouseTableDesc() ([]*ClickHouseDescType, error) {
	var descTypes []*ClickHouseDescType
	rows, err := c.Conn.Query(context.Background(), "DESC "+c.config.DbName+"."+c.config.TableName)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var descType ClickHouseDescType
		err = rows.Scan(&descType.Name, &descType.Type, &descType.DefaultType, &descType.DefaultExpression, &descType.Comment, &descType.CodecExpression, &descType.TTLExpression)
		if err != nil {
			return nil, err
		}

		descTypes = append(descTypes, &descType)
	}
	defer rows.Close()

	return descTypes, nil
}

func (c *client) clickhouseColumns() error {
	descTypes, err := c.clickhouseTableDesc()
	if err != nil {
		return err
	}

	columnType := make(map[string]string)
	for _, item := range descTypes {
		columnType[item.Name] = item.Type
	}
	c.columnsType = columnType

	if len(c.config.Columns) == 0 {
		for _, desc := range descTypes {
			c.config.Columns = append(c.config.Columns, desc.Name)
		}
	}

	for _, column := range c.config.Columns {
		_, ok := columnType[column]
		if !ok {
			return errors.New(column + " not in " + c.config.DbName + " " + c.config.TableName)
		}
	}
	return nil
}
