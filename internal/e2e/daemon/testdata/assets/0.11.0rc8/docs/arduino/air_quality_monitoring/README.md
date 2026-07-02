# Air Quality Monitoring Brick

This Brick provides a Python interface for retrieving and handling real-time air quality data (AQI) from the AQICN API.

## Overview

The Air Quality Monitoring Brick allows you to:

- Query air quality data by city name, geographic coordinates, or current IP location.
- Access structured AQI data, including pollutant details and AQI levels.
- Handle API errors gracefully with custom exceptions.

It converts technical air quality measurements into easy-to-understand categories, such as `Good`, `Moderate`, or `Unhealthy`.

## Features

- Retrieve AQI data by city name, latitude/longitude, or IP-based location.
- Parse and access pollutant concentrations.
- Identify the dominant pollutant affecting air quality.
- Receive AQI levels with health recommendations.
- Robust error handling with descriptive exceptions.
- Simple, object-oriented API for integration into larger projects.

## Prerequisites

You need a free API token from AQICN:
1. Visit [aqicn.org/data-platform/token](https://aqicn.org/data-platform/token/)
2. Register for a free account
3. Save your API token

## Code example and usage

```python
from arduino.app_bricks.air_quality_monitor import AirQualityMonitor

API_TOKEN = "your_token_here"

aqm = AirQualityMonitor(API_TOKEN)
data = aqm.get_air_quality_by_city("Torino")
print(f"AQI in {data.city}: {data.aqi}")
print(f"Dominant pollutant: {data.dominantpol}")
print(f"All pollutants: {data.iaqi}")
```

## Understanding Air Quality Data

The AQI (Air Quality Index) is a number ranging from 0 to 500 that indicates the level of air pollution. The scale ranges from:

- 0-50: Good 
- 51-100: Moderate
- 101-150: Unhealthy for sensitive groups
- 151-200: Unhealthy 
- 201-300: Very unhealthy
- 301-500: Hazardous

The dominant pollutant refers to the specific type of pollution, like PM2.5, ozone, or nitrogen dioxide, that is causing the current air quality level. This helps identify the main source of air quality concerns.

Individual pollutants (IAQI) provide separate measurements for each type of pollutant, which is useful for detailed analysis and understanding the complete picture of air quality.

## Brick Functions
| Function                                          | Example                                    |
| ------------------------------------------------- | ------------------------------------------ |
| get_air_quality_by_city(city:string)              | get_air_quality_by_city("Torino")          |
| get_air_quality_by_coords(lat:float,long:float)   | get_air_quality_by_coords("31.2","121.4")  |
| get_air_quality_by_ip()                           | get_air_quality_by_ip()                    |
