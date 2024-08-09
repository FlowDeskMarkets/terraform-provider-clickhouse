terraform {
  required_providers {
    clickhouse = {
      version = "2.0.0"
      source  = "hashicorp.com/flowdeskmarkets/clickhouse"
    }
  }
}

provider "clickhouse" {
  port     = 9000
  host     = "10.46.0.247"
  username = "sonic"
  password = ""
}

/*
resource "clickhouse_table" "replicated_table" {
  database      = "default"
  name          = "kafka_test"
  engine        = "Kafka"
  engine_params  = ["'sonic-cluster-kafka-bootstrap.internal.sonicwhale.io:9092'", "'test'", "'test'", "'JSONEachRows'"]
  column {
    name = "event_date"
    type = "Date"
    nullable = true
  }
  column {
    name = "event_type"
    type = "Int32"
  }
  column {
    name = "article_id"
    type = "Int32"
  }
  column {
    name = "title"
    type = "String"
  }

  settings = {
    kafka_thread_per_consumer = 1
    kafka_num_consumers = 8
  }

}
*/

resource "clickhouse_table" "t2" {
  database      = "default"
  name          = "Replicated_test"
  engine        = "ReplacingMergeTree"
  engine_params = []
  comment       = "hi!"
  cluster       = "main"
  column {
    name = "event_date"
    type = "Date"
  }
  column {
    name               = "event_type"
    type               = "Int32"
    default_kind       = "DEFAULT"
    default_expression = 5
  }
  column {
    name = "event_type_2"
    type = "Int32"
  }
  column {
    name = "article.id"
    type = "Int32"
  }
  column {
    name               = "article.title"
    type               = "String"
    default_kind       = "DEFAULT"
    default_expression = "'yo'"

  }
  order_by = ["event_date", "event_type"]
  partition_by {
    by                 = "event_date"
    partition_function = "sipHash64"
    mod                = "1000"
  }
  partition_by {
    by                 = "event_type"
    partition_function = "sipHash64"

  }
  index {
    name        = "test_index"
    expression  = "[event_type, event_type_2]"
    type        = "minmax"
    granularity = 10000
  }
}

/*
resource "clickhouse_view" "test_view" {
  database      = "default"
  name          = "test_view"
  cluster="main"
query = "select * FROM default.shop_settings LIMIT 10"
}
*/
/*
resource "clickhouse_view" "test_materialized_view" {
  database      = "default"
  name          = "test_materialized_view"
  materialized = true
  to_table = "test_view"
  cluster="main"
query = "select * from default.nx_refunds LIMIT 10"
}

*/
