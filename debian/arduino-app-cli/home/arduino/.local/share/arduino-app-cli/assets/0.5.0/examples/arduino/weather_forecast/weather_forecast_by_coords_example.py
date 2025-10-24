# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Weather Forecast by coordinates"
from arduino.app_bricks.weather_forecast import WeatherForecast

forecaster = WeatherForecast()

forecast = forecaster.get_forecast_by_coords(latitude="45.0703", longitude="7.6869")
print(f"The weather forecast says it will be {forecast.category} ({forecast.description}).")
