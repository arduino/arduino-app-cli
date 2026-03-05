# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect speech from microphone, different providers"
# EXAMPLE_REQUIRES = "Requires an USB microphone connected to the Arduino board."
from arduino.app_bricks.cloud_asr import CloudASR, CloudProvider

cloud_asr_openai = CloudASR(provider=CloudProvider.OPENAI_TRANSCRIBE, api_key="YOUR__OPENAI_API_KEY")
text = cloud_asr_openai.transcribe()
print(f"Detected speech: {text}")

cloud_asr_google = CloudASR(provider=CloudProvider.GOOGLE_SPEECH, api_key="YOUR_GOOGLE_API_KEY")
text = cloud_asr_google.transcribe()
print(f"Detected speech: {text}")
