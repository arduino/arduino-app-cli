# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Weather Forecast by city name"
from arduino.app_bricks.weather_forecast import WeatherForecast

forecaster = WeatherForecast()

city = "Turin"
forecast = forecaster.get_forecast_by_city(city)
print(f"The weather forecast for {city} says it will be {forecast.category} ({forecast.description}).")
