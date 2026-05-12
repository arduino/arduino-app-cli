# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Stream synthesized PCM audio chunks as they arrive"

from arduino.app_bricks.tts import TextToSpeech

tts = TextToSpeech()

with tts.synthesize_pcm_stream("Hello, Arduino world!") as stream:
    with open("synthesized_speech.pcm", "wb") as f:
        for chunk in stream:
            f.write(chunk)
