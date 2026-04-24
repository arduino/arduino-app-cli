# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Transcribe a wav file"
# EXAMPLE_REQUIRES = "Requires a WAV file with a voice recording"
from arduino.app_bricks.asr import AutomaticSpeechRecognition


asr = AutomaticSpeechRecognition()
with open("recording_01.wav", "rb") as wav_file:
    text = asr.transcribe_wav(wav_file.read())
    print(f"Transcription: {text}")
