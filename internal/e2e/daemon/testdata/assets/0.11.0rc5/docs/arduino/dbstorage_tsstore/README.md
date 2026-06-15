# Database - Time Series Brick

This brick helps you manage and store time series data efficiently using InfluxDB.

## Overview

The Database - Time series brick allows you to:

- Efficiently store and retrieve time series data
- Use a simple API for writing and reading time series measurements
- Handle database connections automatically
- Integrate your projects easily with InfluxDB
- Use methods for querying and managing stored data
- Handle errors and manage resources robustly

It provides a refined interface for working with time series data, automatically managing InfluxDB connections and providing flexible querying capabilities with time ranges, aggregation functions, and data retention policies.

## Features

- Automatic data retention management with configurable retention periods
- Flexible time range queries with relative periods (e.g., `-1d`, `-2h`) or absolute timestamps
- Data aggregation support with functions like *mean*, *max*, *min*, and *sum*
- Configurable measurement organization and field naming
- Thread-safe operations for concurrent access
- Built-in validation for time parameters and aggregation settings

## Code example and usage

Instantiate a new class to open a database connection:

```python
import time
from arduino.app_bricks.dbstorage_tsstore import TimeSeriesStore

db = TimeSeriesStore()
db.start()

db.write_sample("temp", 21)
db.write_sample("hum", 45)
time.sleep(1)

last_temp = db.read_last_sample("temp")
last_hum = db.read_last_sample("hum")
print(f"Last temperature: {last_temp}")
print(f"Last humidity: {last_hum}")

db.stop()
```

## Understanding Time Series Operations

The TimeSeriesStore organizes data using InfluxDB's measurement and field structure, where measurements work as containers for related metrics and fields represent individual sensor readings or data points. Each data point includes a timestamp, allowing for precise time-based queries and analysis.

The brick supports flexible time range specifications using relative periods, such as `-1d` for the last day or `-2h` for the last two hours, as well as absolute timestamps in RFC 3339 format. Data retention is automatically managed based on the configured retention period, allowing for controlled storage usage while maintaining relevant historical data.