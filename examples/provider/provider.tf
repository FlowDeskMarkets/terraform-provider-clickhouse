terraform {
  required_providers {
    clickhouse = {
      version = "2.0.0"
      source  = "hashicorp.com/flowdeskmarkets/clickhouse"
    }
  }
}


provider "clickhouse" {
  port     = 8123
  host     = "127.0.0.1"
  username = "default"
  password = ""

  # Optional: Timeout and retry settings for idle/sleeping services
  dial_timeout = 60
  max_retries  = 5
  retry_delay  = 10
}
