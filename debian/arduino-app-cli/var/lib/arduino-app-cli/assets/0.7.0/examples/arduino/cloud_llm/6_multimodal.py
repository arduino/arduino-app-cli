# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with an Multimodal LLM"
# EXAMPLE_REQUIRES = "Requires a valid API key to a cloud LLM service."

from arduino.app_bricks.cloud_llm import CloudLLM, CloudModel
from arduino.app_utils import App
import time

llm = CloudLLM(
    model=CloudModel.GOOGLE_GEMINI,
    api_key="YOUR_API_KEY",  # Replace with your actual API key
)


def ask_prompt():
    print(
        llm.chat(message="Describe the following image. Provide a bullet-point summary of the discovered objects", images=["path/to/your/image.jpg"])
    )
    time.sleep(60)


App.run(ask_prompt)
