# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with a Local LLM"
# EXAMPLE_REQUIRES = "Models must be downloaded and available locally."

from arduino.app_bricks.llm import LargeLanguageModel
from arduino.app_utils import App

llm = LargeLanguageModel()


def ask_prompt():
    prompt = "Hi, what can you do as an AI assistant?"
    print(llm.chat(prompt))
    print()
    raise StopIteration


App.run(ask_prompt)
