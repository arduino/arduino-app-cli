# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Chat with an LLM using Tool Calling"
# EXAMPLE_REQUIRES = "Requires a valid API key to a cloud LLM service."

from arduino.app_bricks.cloud_llm import CloudLLM, tool
from arduino.app_utils import App


@tool
def get_current_weather(location: str) -> str:
    """
    Get the current weather in a given location.
    The output is a string with a summary of the weather.
    """
    if "boston" in location.lower():
        return "The current weather in Boston is -5°C and rainy."
    elif "paris" in location.lower():
        return "The current weather in Paris is 8°C and rainy."
    elif "turin" in location.lower():
        return "The current weather in Turin is 8°C and rainy."
    else:
        return f"Sorry, I do not have real-time weather data for {location}. Assuming it's a sunny day!"


llm = CloudLLM(
    model="google:gemini-2.5-flash",
    api_key="YOUR_API_KEY",  # Replace with your actual API key
    tools=[get_current_weather],
)


def ask_prompt():
    prompt = input("Enter your prompt (or type 'exit' to quit): ")
    if prompt.lower() == "exit":
        raise StopIteration()
    print(llm.chat(prompt))
    print()


App.run(ask_prompt)
