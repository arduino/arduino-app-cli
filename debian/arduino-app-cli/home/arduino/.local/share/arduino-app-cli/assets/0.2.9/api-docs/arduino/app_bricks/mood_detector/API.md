# mood_detector API Reference

## Index

- Class `MoodDetector`

---

## `MoodDetector` class

```python
class MoodDetector()
```

A class to detect mood based on text sentiment analysis.

As example, it can classify text as positive, negative, or neutral.

Sentence: 'I love this board!' -> Analysis: positive
Sentence: 'the weather is awful' -> Analysis: negative
Sentence: 'I am sad today' -> Analysis: negative
Sentence: 'the temperature is 25' -> Analysis: neutral

NOTE: Detector support English language only.

### Methods

#### `get_sentiment(text: str)`

Analyze the sentiment of the provided text and return the mood.

##### Parameters

- **text** (*str*): The input text to analyze.

##### Returns

- (*str*): The mood of the text, which can be 'positive', 'negative', or 'neutral'.

