# filebeat-clickhouse
The output for filebeat support push events to ClickHouse，You need to recompile filebeat with the ClickHouse output.  
Compared with the combination of KafKa+ZooKeeper+ClickHouse(MATERIALIZED VIEW), it's simpler and less use of system resources in scenarios with small data num.  
Currently compiled based on version `filebeat version 8.6.0, libbeat 8.6.0`.  

