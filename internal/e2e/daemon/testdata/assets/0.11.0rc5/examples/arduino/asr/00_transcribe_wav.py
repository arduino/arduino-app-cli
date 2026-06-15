# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Transcribe a wav file"
# EXAMPLE_REQUIRES = "Requires a WAV file with a voice recording"
from arduino.app_bricks.asr import AutomaticSpeechRecognition


with open("recording_01.wav", "rb") as wav_file:
    asr = AutomaticSpeechRecognition(wav_file.read())
    text = asr.transcribe()
    print(f"Transcription: {text}")
