# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Transcribe a wav file and stream the results"
# EXAMPLE_REQUIRES = "Requires a WAV file with a voice recording"
from arduino.app_bricks.asr import AutomaticSpeechRecognition


with open("recording_01.wav", "rb") as wav_file:
    asr = AutomaticSpeechRecognition(wav_file.read())
    with asr.transcribe_stream() as stream:
        for chunk in stream:
            match chunk.type:
                case "partial_text":
                    print(f"Partial: {chunk.data}")
                case "full_text":
                    print(f"Final: {chunk.data}")
                    break
