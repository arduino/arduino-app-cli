# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with an LLM"
# EXAMPLE_REQUIRES = "Requires a valid API key to a cloud LLM service."

from arduino.app_bricks.cloud_llm import CloudLLM
from arduino.app_utils import App

llm = CloudLLM(
    api_key="YOUR_API_KEY",  # Replace with your actual API key
)


def ask_prompt():
    prompt = input("Enter your prompt (or type 'exit' to quit): ")
    if prompt.lower() == "exit":
        raise StopIteration()
    print(llm.chat(prompt))
    print()


App.run(ask_prompt)
