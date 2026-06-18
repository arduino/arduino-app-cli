# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with a Local VLM"
# EXAMPLE_REQUIRES = "Models must be downloaded and available locally."

from arduino.app_bricks.vlm import VisionLanguageModel

vlm = VisionLanguageModel()

for chunk in vlm.chat_stream("Describe the image.", images=["chair.jpg"]):
    print(chunk, end="", flush=True)
