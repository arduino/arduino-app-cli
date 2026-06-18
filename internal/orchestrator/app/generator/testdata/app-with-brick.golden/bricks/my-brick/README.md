# a-brick-name

Your Brick **a-brick-name** is ready!

Custom Bricks are modular components that add reusable functionality to your app.

## How to use it

Write your Python class inside `bricks/my-brick/__init__.py` and add the `@brick` decorator.
```python
# bricks/my-brick/__init__.py
from arduino.app_utils import App, brick, Logger
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

```

You can then import and use your Brick in your main application code:
```python
# python/main.py
from arduino.app_utils import App
from greeter import Greeter

g = Greeter()

App.run()
```

When `App.run()` is executed, the framework automatically manages the application lifecycle and threading:
```
======== App is starting ============================
2026-06-18 09:47:27.250 INFO - [MainThread] App:  App started
2026-06-18 09:47:27.247 INFO - [MainThread] GreeterBrick:  Starting Greeter
2026-06-18 09:47:27.249 INFO - [Greeter.loop] GreeterBrick:  Hello, World!
2026-06-18 09:47:28.251 INFO - [Greeter.loop] GreeterBrick:  Hello, World!
2026-06-18 09:47:29.251 INFO - [Greeter.loop] GreeterBrick:  Hello, World!
2026-06-18 09:47:57.797 INFO - [MainThread] App:  App is shutting down
2026-06-18 09:47:57.798 INFO - [MainThread] GreeterBrick:  Stopping Greeter
======== App shutdown completed =====================
```

## What's in the brick folder

your brick is created under the `bricks/my-brick` folder that contains the following files:
- `__init__.py` Your core Python code. This is what gets imported.
- `brick_config.yaml` Brick identity, variables, and configuration
- `brick_compose.yaml` A Docker Compose file for adding custom containers (if your Brick requires external services like a database).

## Configure your Brick

The `brick_config.yaml` file defines your Brick's identity and variables it needs to run.

For example, to add a configuration variable called `YOUR_NAME` to your Brick, define it under the variables section:
```yaml
id: my-brick
name: a-brick-name
variables:
    - name: YOUR_NAME
     description: A name to greet
```
Note: Variables defined in this file are automatically injected into your Brick as environment variables at runtime.

You can then read this variable inside your Python code using the standard os module.

``` python
import os
name = os.getenv("YOUR_NAME")
```

## Next

Replace this README with your Brick's docs: what it does, inputs, outputs, and a usage example.
[See Documentation on Docs](https://docs.arduino.cc/software/app-lab/bricks/about-bricks/)