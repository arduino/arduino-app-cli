# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Transcribe audio from microphone"
# EXAMPLE_REQUIRES = "Requires a microphone device"
from arduino.app_bricks.asr import AutomaticSpeechRecognition
from arduino.app_peripherals.microphone import Microphone


mic = Microphone()
mic.start()

asr = AutomaticSpeechRecognition(mic)
text = asr.transcribe(duration=5)
print(f"Transcription: {text}")

mic.stop()
