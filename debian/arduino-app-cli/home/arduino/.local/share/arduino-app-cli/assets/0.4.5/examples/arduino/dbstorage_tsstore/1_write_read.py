# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Store and read data using TimeSeriesStore"
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
