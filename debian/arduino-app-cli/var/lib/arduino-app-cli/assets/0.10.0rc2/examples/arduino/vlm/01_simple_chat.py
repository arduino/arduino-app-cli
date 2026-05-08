# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with a Local VLM"
# EXAMPLE_REQUIRES = "Models must be downloaded and available locally."

from arduino.app_bricks.vlm import VisionLanguageModel

vlm = VisionLanguageModel()

print(vlm.chat("Describe the image.", images=["chair.jpg"]))
