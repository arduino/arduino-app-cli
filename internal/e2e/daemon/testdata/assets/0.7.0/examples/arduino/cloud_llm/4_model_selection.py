# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "LLM Model Selection"
# EXAMPLE_REQUIRES = "Requires a valid API key to a cloud LLM service."

from arduino.app_bricks.cloud_llm import CloudLLM
from arduino.app_utils import App

llm = CloudLLM(
    model="google:gemini-2.5-flash",  # or CloudModel.GOOGLE_GEMINI
    api_key="YOUR_API_KEY",  # Replace with your actual API key
)


def ask_prompt():
    prompt = input("Enter your prompt (or type 'exit' to quit): ")
    if prompt.lower() == "exit":
        raise StopIteration()
    print(llm.chat(prompt))
    print()


App.run(ask_prompt)
