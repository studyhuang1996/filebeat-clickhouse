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
   ==> release 中获取
