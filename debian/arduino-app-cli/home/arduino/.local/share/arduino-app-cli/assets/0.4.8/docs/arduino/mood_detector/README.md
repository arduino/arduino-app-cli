# Mood detector brick

This brick analyzes text sentiment to detect the mood expressed.
It classifies text as positive, negative, or neutral.

Examples:
- "I love this board!" -> positive
- "The weather is awful" -> negative
- "I am sad today" -> negative
- "The temperature is 25" -> neutral

## Code example and usage

```python
from arduino.app_bricks.mood_detector import MoodDetector

mood_detection = MoodDetector()

# Output: positive
print(mood_detection.get_sentiment("this application is nice"))
```
