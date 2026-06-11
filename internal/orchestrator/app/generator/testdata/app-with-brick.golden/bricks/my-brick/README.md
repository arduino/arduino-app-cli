# a-brick-name

Your a-brick-name Brick is ready.
Custom Bricks are modular components that add reusable functionality to your app.

## How to use it

Write Python in `bricks/my-brick/__init__.py`
```
# bricks/my-brick/__init__.py
def greet(name):
    return f"Hello {name}, I am the a-brick-name brick"
```

Import it  into `python/main.py`:
```
# python/main.py
from my-brick import greet

print(greet("world"))
```

## Configure your Brick:
`brick_config.yaml` defines your Brick's identity and variables:
```yaml
id: my-brick
name: a-brick-name
description: "I am a custom brick"
variables:
    - name: API_KEY
     description: Your key
     secret: true
```

## What's in the folder
```
__init__.py — your Python code
brick_config.yaml — Brick identity, variables, and configuration
brick_compose.yaml — Docker Compose file for adding containers
```

## Next
Replace this README with your Brick's docs: what it does, inputs, outputs, and a usage example.
[See Documentation on Docs](https://docs.arduino.cc/software/app-lab/bricks/about-bricks/)