# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with a Local LLM"
# EXAMPLE_REQUIRES = "Models must be downloaded and available locally."

from arduino.app_bricks.llm import LargeLanguageModel
from arduino.app_utils import App

llm = LargeLanguageModel(model="genie:qwen3:4b")


def ask_prompt():
    prompt = input("Enter your prompt (or type 'exit' to quit): ")
    if prompt.lower() == "exit":
        raise StopIteration()
    for chunk in llm.chat_stream(prompt):
        print(chunk, end="", flush=True)
    print()


App.run(ask_prompt)
