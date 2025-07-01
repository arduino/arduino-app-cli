# Database Storage - Time Series Store

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
