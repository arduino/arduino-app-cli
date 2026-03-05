# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect speech from microphone"
# EXAMPLE_REQUIRES = "Requires an USB microphone connected to the Arduino board."
from arduino.app_bricks.cloud_asr import CloudASR

cloud_asr = CloudASR(
    api_key="YOUR_API_KEY",  # Replace with your actual API key
)

text = cloud_asr.transcribe()
print(f"Detected speech: {text}")
