# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect speech from microphone, event stream"
# EXAMPLE_REQUIRES = "Requires an USB microphone connected to the Arduino board."
from arduino.app_bricks.cloud_asr import CloudASR

cloud_asr = CloudASR(
    api_key="YOUR_API_KEY",  # Replace with your actual API key
)

with cloud_asr.transcribe_stream() as stream:
    print("Say 'stop' to stop the transcription.")

    for event in stream:
        print(f"{event.type}: {event.data}")
        if event.type == "text" and (event.data or "").lower() == "stop":
            print("Stopping transcription stream...")
            break
