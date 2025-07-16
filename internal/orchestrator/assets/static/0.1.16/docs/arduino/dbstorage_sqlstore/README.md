# Database Storage - SQLStore

This brick helps you manage SQLite databases easily by providing a simple interface for creating tables, inserting data, and handling database connections.

## Features

- Simple API for SQLite database operations
- Create tables with custom schemas
- Insert, update, and delete records
- Query data with flexible filters
- Automatic connection management
- Error handling for common database issues

## Code example and usage

Instantiate a new class to open (or create a new database)

```python
from arduino.app_bricks.dbstorage_sqlstore import SQLStore

db = SQLStore("example.db")
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
