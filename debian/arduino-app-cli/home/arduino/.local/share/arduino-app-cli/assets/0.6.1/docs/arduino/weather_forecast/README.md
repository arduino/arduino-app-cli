# Weather Forecast

Streamlined online weather API for retrieving forecasts by city name or geographic coordinates.
Forecasts are provided by [`open-meteo.com`](https://open-meteo.com/).

## Overview

The Weather Forecast brick allows you to:

- Retrieve current and forecast weather data
- Query by city name (e.g. `"Rome"`, `"New York"`)
- Query by geographic coordinates (latitude & longitude)

It converts technical weather codes into simple categories, such as `sunny`, `cloudy`, `rainy`, `snowy`, or `foggy`, for easy integration with displays and applications. You can get weather forecasts for any city worldwide, specify forecast duration, and access weather codes, descriptions, and simplified categories with no API key required using the [`open-meteo.com`](https://open-meteo.com/) service.

## Features

- Supports multi-day forecasts using the `forecast_days` parameter
- Uses WMO (World Meteorological Organization) standard weather codes
- Automatic city geocoding functionality for location lookup
- Free access for non-commercial use with no API key requirements with [`open-meteo.com`](https://open-meteo.com/)

## Code example and usage

Here is an example for querying a 1-day weather forecast for a specific city:

```python
from arduino.app_bricks.weather_forecast import WeatherForecast

forecaster = WeatherForecast()

forecast = forecaster.get_forecast_by_city('Turin')
```

It is also possible to query by geographic coordinates:

```python
forecast = forecaster.get_forecast_by_coords(latitude = "45.0703", longitude = "7.6869")
```

You can specify the number of forecast days using the `forecast_days` parameter.

```python
forecast = forecaster.get_forecast_by_city(city='Turin', forecast_days=2)
```

## Understanding Weather Data

The WeatherData object includes three key pieces of information for each forecast. The code provides the official WMO (World Meteorological Organization) weather code, which is a standardized number representing specific weather conditions.

The description provides an easily understandable explanation of the weather condition, such as `Partly cloudy` or `Heavy rain`. The category simplifies weather conditions into five basic types: *sunny*, *cloudy*, *rainy*, *snowy*, or *foggy*, making it easy to create visual displays or simple decision logic.
