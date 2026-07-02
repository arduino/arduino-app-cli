# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Get air quality by city"
# EXAMPLE_REQUIRES = "Requires an AQICN API token (https://aqicn.org/data-platform/token/)."

from arduino.app_bricks.air_quality_monitoring import AirQualityMonitoring
from arduino.app_utils import App

monitor = AirQualityMonitoring(token="YOUR_AQICN_TOKEN")

city = "Turin"
data = monitor.get_air_quality_by_city(city)
print(f"Air quality in {data.city}: AQI = {data.aqi}, dominant pollutant: {data.dominantpol}")

App.run()
