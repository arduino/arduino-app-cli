# Keyword Spotting Brick

Brick for keyword spotting using a pre-trained model. It processes audio input to detect specific keywords or phrases.
  Brick is designed to work with pre-trained models provided by framework or with custom audio classification models trained on Edge Impulse platform.

## Code example and usage

```python
from arduino.app_bricks.keyword_spotting import KeywordSpotting

keyword_spotting = KeywordSpotting()
out = keyword_spotting.classify_from_file("path/to/your/audio.wav")
print(f"Keyword Spotting Results: {out}")
```
