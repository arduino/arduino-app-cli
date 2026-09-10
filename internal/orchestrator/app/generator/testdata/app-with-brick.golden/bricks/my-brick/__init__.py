from arduino.app_utils import brick, Logger
import time

logger = Logger("GreeterBrick")


@brick
class Greeter:
    def __init__(self, name="World"):
        self.name = name

    def start(self):
        logger.info("Starting Greeter")

    def stop(self):
        logger.info("Stopping Greeter")

    # This is a non-blocking method that will be called repeatedly
    def loop(self):
        logger.info(f"Hello, {self.name}!")
        time.sleep(1)
