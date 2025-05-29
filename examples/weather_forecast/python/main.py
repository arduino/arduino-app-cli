from arduino.app_bricks.mcu import Server
from arduino.app_bricks.weather_forecast import WeatherForecast
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

forecaster = WeatherForecast()

def get_weather_forecast(city: str) -> str | None:
    try:
        forecast = forecaster.get_forecast_by_city(city)
    except Exception as e:
        logger.error(f"Failed to get weather forecast for {city}: {e}")
        return None

    logger.info(f"Weather forecast for {city}: {forecast.description}")

    return forecast.category

srv = Server()
srv.register("get_weather_forecast", get_weather_forecast)
srv.loop_forever()
