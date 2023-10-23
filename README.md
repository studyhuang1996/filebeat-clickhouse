# filebeat-clickhouse
> 由于filebeat的output插件不支持clickhouse，当前需求需要把 log 文件中的数据直接同步 Clickhouse 中, 尝试找了一些其他的插件，不太符合
> 自身的需求和预期，所以根据前人经验重写 clickhouse插件，目前测试是ok的，后续会持续优化，由于首次写 go ，有不对的，欢迎大家指正。

> filebeat 版本是基于当前最新版本 8.10.2 编译的


## 使用方式

* 根据此项目手动编译
  * 1、下载 beats 项目
  ```
    git  clone https://github.com/elastic/beats.git
  ```
  * 2、获取本项目的 filebeat-clickhouse 插件
  ```
    go get github.com/studyhuang1996/filebeat-clickhouse.git
  ```
  * 3、beat中引入该项目
  ```
    cd {your beats directory}/github.com/elastic/beats/libbeat/publisher/includes;
     vi includes.go;
      
      import (
           ...
           _ "github.com/studyhuang1996/filebeat-clickhouse"
     )

  ```
  * 4、编译
  ```
  cd {your beats directory}/github.com/elastic/beats/filebeat
    make 
  或者 使用 mage build  
   ```
  beats 如何编译 ，可查阅[官方文档](https://www.elastic.co/guide/en/beats/devguide/current/beats-contributing.html)



* 使用项目提供的已经编译好的包
   ==> release 中获取已经编译好的二进制文件进行处理


## 使用配置
 * 原始文档, 格式化成JSON 结构
  ```
   1、原始文本
   { "timestamp": "2023-08-22 09:07:16", "id": 0, "class": "connection", "event": "connect", "connection_id": 9, 
   "account": { "user": "root", "host": "" }, "login": { "user": "root", "os": "", "ip": "127.0.0.1", "proxy": "" }, 
   "connection_data": { "connection_type": "ssl", "status": 0, "db": "", "connection_attributes": { "_runtime_version": "11.0.8", "_client_version": "8.0.25", "_client_license": "GPL", "_runtime_vendor": "JetBrains s.r.o.", "_client_name": "MySQL Connector\/J" } } }
   2、格式化成 json 的文本
   { 
  "@timestamp": "2023-10-23T09:54:22.811Z",
  "@metadata": {
    "beat": "filebeat",
    "type": "_doc",
    "version": "8.12.0"
  },
  "login": {
    "ip": "*****",
    "proxy": "",
    "user": "root",
    "os": ""
  },
  "timestamp": "2023-08-22 09:07:16",
  "connection_id": 9,
  "host": {
    "name": "CHINAMI-H2SHAT4"
  },
  "class": "connection",
  "account": {
    "host": "",
    "user": "root"
  },
  "CONNECTION_ATTRIBUTES": {
    "_client_version": "8.0.25",
    "_client_license": "GPL",
    "_runtime_vendor": "JetBrains s.r.o.",
    "_client_name": "MySQL Connector/J",
    "_runtime_version": "11.0.8"
  },
  "ADD_TEST": "新增字段"
  }
  ```
 * filebeat.yml 配置
```
filebeat.inputs:
- type: log
  enabled: true
  paths:
      # 日志实际路径地址
    - C:\Users\Administrator\Desktop\test.log
  json.keys_under_root: true
  json.overwrite_keys: true
  json.add_error_key: true



# ---------------------------- clickhouse Output ----------------------------
output.clickhouse:
 #ck的地址 默认9000 端口 如果有 参数可直接加在 127.0.0.1:9000?useUnicode=true&characterEncoding=utf-8
  host: "127.0.0.1:9000"
  username: "default"
  password: "123456"
  #数据库名称
  db_name: "default"
  #表名
  table_name: "AUDIT_TEST"
  # 可不填，填写表示只写入指定的列，不填表示写入所有列
  columns: ["CLASS","EVENT"]
  batch_size: 2048

# ================================= Processors =================================
processors:
  - decode_json_fields:
      fields: ["message"]
      target: ""
      overwrite_keys: true
      add_error_key: true        
  - add_fields:
      target: ''
      fields:
        ADD_TEST: '新增字段'
```
