# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Transcribe audio from microphone and stream the results"
# EXAMPLE_REQUIRES = "Requires a microphone device"
from arduino.app_bricks.asr import AutomaticSpeechRecognition
from arduino.app_peripherals.microphone import Microphone


mic = Microphone()
mic.start()

asr = AutomaticSpeechRecognition(mic)
with asr.transcribe_stream(duration=5) as stream:
    for chunk in stream:
        match chunk.type:
            case "partial_text":
                print(f"Partial: {chunk.data}")
            case "full_text":
                print(f"Final: {chunk.data}")
                break

mic.stop()
