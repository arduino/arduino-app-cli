# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with an Ollama model"

from arduino.app_bricks.cloud_llm import CloudLLM
from arduino.app_utils import App
import time

llm = CloudLLM(
    model="qwen3.5:0.8b",  # Replace with the actual model name you want to use. Model must be available in your Ollama instance.
    base_url="http://localhost:11434/v1",  # Default Ollama address
    system_prompt="You are a helpful assistant that provides concise answers to questions about historical figures.",
)


def ask_prompt():
    print("\n----- Sending prompt to the model -----")
    for chunk in llm.chat_stream(message="Who was Giuseppe Verdi?"):
        print(chunk, end="", flush=True)
    print("\n----- Response complete -----")
    time.sleep(60)


App.run(ask_prompt)
