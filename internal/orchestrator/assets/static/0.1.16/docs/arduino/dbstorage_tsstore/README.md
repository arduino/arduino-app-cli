# Database Storage - Time Series Store

This brick helps you manage and store time series data efficiently using an Influx DB.

## Features

- Efficient storage and retrieval of time series data
- Simple API for creating tables and inserting data
- Automatic handling of database connections
- Easy integration with Influx DB
- Methods for querying and managing stored data
- Robust error handling and resource management

## Code example and usage

Instantiate a new class to open database connection.

```python
from arduino.app_bricks.dbstorage_tsstore import TimeSeriesStore

db = TimeSeriesStore()
# ... Do work

# Close database
db.close()
```

to create a new table

```python
# Create a table
columns = {
    "id": "INTEGER PRIMARY KEY",
    "name": "TEXT",
    "age": "INTEGER"
}
db.create_table("users", columns)
```

insert new data in a table

```python
# Insert data
data = {
    "name": "Alice",
    "age": 30
}
db.store("users", data)
```
