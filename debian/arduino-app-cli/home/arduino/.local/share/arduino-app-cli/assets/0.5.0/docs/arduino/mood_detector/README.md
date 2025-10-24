# Mood Detector Brick

This directory contains the implementation of the Mood Detector Brick, which classifies text sentiment as positive, negative, or neutral using the NLTK VADER analyzer bundled with the Brick assets (no external download required at runtime).

## Overview

The Mood Detector Brick analyzes a sentence and returns its overall mood: "positive", "negative", or "neutral". It is lightweight, runs locally, and requires no internet connection.

Examples:
- "I love this board!" -> positive
- "The weather is awful" -> negative
- "I am sad today" -> negative
- "The temperature is 25" -> neutral


## Features

- Classifies text as positive, negative, or neutral.
- Runs locally; no external services required.
- Case-insensitive; robust to basic punctuation.
- Sensible defaults for edge cases: empty/whitespace -> neutral; non-English text -> typically neutral.
- Simple API: `MoodDetector.get_sentiment(text) -> str`.

## Code example and usage

```python
from arduino.app_bricks.mood_detector import MoodDetector

mood = MoodDetector()

print(mood.get_sentiment("This is a wonderful and amazing product!"))  # positive
print(mood.get_sentiment("I am feeling very sad and disappointed today."))  # negative
print(mood.get_sentiment("The report will be ready by 5 PM."))  # neutral

# Edge cases
print(mood.get_sentiment(""))                 # neutral (empty input)
print(mood.get_sentiment("Questo è bello"))   # neutral (non-English)
```
