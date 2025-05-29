# Weather Forecast brick

Streamlined online weather API for retrieving forecasts by city name or geographic coordinates.
Forecasts are provided by open-meteo.com.

## Features

- Retrieve current and forecast weather data
- Query by city name (e.g. `"Rome"`, `"New York"`)
- Query by geographic coordinates (latitude & longitude)

## Code example and usage

Here is an example for quering 1d weather forcast for a specific city

```python
from arduino.app_bricks.weather_forecast import WeatherForecast

forecaster = WeatherForecast()

forecast = forecaster.get_forecast_by_city('Turin')
```

it is possible to query also by geographic coordinates

```python
forecast = forecaster.get_forecast_by_coords(latitude = "45.0703", longitude = "7.6869")
```

You can specify the number of forecast days using the `forecast_days` parameter.

```python
forecast = forecaster.get_forecast_by_city(city='Turin', forecast_days=2)
```
