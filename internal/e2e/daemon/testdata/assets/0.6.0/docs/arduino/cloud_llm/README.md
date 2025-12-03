# Cloud LLM Brick

The Cloud LLM Brick provides a seamless interface to interact with cloud-based Large Language Models (LLMs) such as OpenAI's GPT, Anthropic's Claude, and Google's Gemini. It abstracts the complexity of REST APIs, enabling you to send prompts, receive responses, and maintain conversational context within your Arduino projects.

## Overview

This Brick acts as a gateway to powerful AI models hosted in the cloud. It is designed to handle the nuances of network communication, authentication, and session management. Whether you need a simple one-off answer or a continuous conversation with memory, the Cloud LLM Brick provides a unified API for different providers.

## Features

- **Multi-Provider Support**: Compatible with major LLM providers including Anthropic (Claude), OpenAI (GPT), and Google (Gemini).
- **Conversational Memory**: Built-in support for windowed history, allowing the AI to remember context from previous exchanges.
- **Streaming Responses**: Receive text chunks in real-time as they are generated, ideal for responsive user interfaces.
- **Configurable Behavior**: Customize system prompts, temperature (creativity), and request timeouts.
- **Simple API**: Unified `chat` and `chat_stream` methods regardless of the underlying model provider.

## Prerequisites

- **Internet Connection**: The board must be connected to the internet to reach the LLM provider's API.
- **API Key**: A valid API key for the chosen service (e.g., OpenAI API Key, Anthropic API Key).
- **Python Dependencies**: The Brick relies on LangChain integration packages (`langchain-anthropic`, `langchain-openai`, `langchain-google-genai`).

## Code Example and Usage

### Basic Conversation

This example initializes the Brick with an OpenAI model and performs a simple chat interaction. 

**Note:** The API key is not hardcoded. It is retrieved automatically from the **Brick Configuration** in App Lab.

```python
import os
from arduino.app_bricks.cloud_llm import CloudLLM, CloudModel
from arduino.app_utils import App

# Initialize the Brick (API key is loaded from configuration)
llm = CloudLLM(
    model=CloudModel.OPENAI_GPT,
    system_prompt="You are a helpful assistant for an IoT device."
)

def simple_chat():
    # Send a prompt and print the response
    response = llm.chat("What is the capital of Italy?")
    print(f"AI: {response}")

# Run the application
App.run(simple_chat)
```

### Streaming with Memory

This example demonstrates how to enable conversational memory and process the response as a stream of tokens.

```python
from arduino.app_bricks.cloud_llm import CloudLLM, CloudModel
from arduino.app_utils import App

# Initialize with memory enabled (keeps last 10 messages)
# API Key is retrieved automatically from Brick Configuration
llm = CloudLLM(
    model=CloudModel.ANTHROPIC_CLAUDE
).with_memory(max_messages=10)

def chat_loop():
    while True:
        user_input = input("You: ")
        if user_input.lower() in ["exit", "quit"]:
            break
            
        print("AI: ", end="", flush=True)
        
        # Stream the response token by token
        for token in llm.chat_stream(user_input):
            print(token, end="", flush=True)
        print() # Newline after response

App.run(chat_loop)
```

## Configuration

The Brick is initialized with the following parameters:

| Parameter       | Type                  | Default                       | Description                                                                                                                              |
| :-------------- | :-------------------- | :---------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------- |
| `api_key`       | `str`                 | `os.getenv("API_KEY")`        | The authentication key for the LLM provider. **Recommended:** Set this via the **Brick Configuration** menu in App Lab instead of code. |
| `model`         | `str` \| `CloudModel` | `CloudModel.ANTHROPIC_CLAUDE` | The specific model to use. Accepts a `CloudModel` enum or its string value.                                                              |
| `system_prompt` | `str`                 | `""`                          | A base instruction that defines the AI's behavior and persona.                                                                           |
| `temperature`   | `float`               | `0.7`                         | Controls randomness. `0.0` is deterministic, `1.0` is creative.                                                                          |
| `timeout`       | `int`                 | `30`                          | Maximum time (in seconds) to wait for a response.                                                                                        |

### Supported Models

You can select a model using the `CloudModel` enum or by passing the corresponding raw string identifier.

| Enum Constant                 | Raw String ID              | Provider Documentation                                                      |
| :---------------------------- | :------------------------- | :-------------------------------------------------------------------------- |
| `CloudModel.ANTHROPIC_CLAUDE` | `claude-3-7-sonnet-latest` | [Anthropic Models](https://docs.anthropic.com/en/docs/about-claude/models)  |
| `CloudModel.OPENAI_GPT`       | `gpt-4o-mini`              | [OpenAI Models](https://platform.openai.com/docs/models)                    |
| `CloudModel.GOOGLE_GEMINI`    | `gemini-2.5-flash`         | [Google Gemini Models](https://ai.google.dev/gemini-api/docs/models/gemini) |

## Methods

- **`chat(message)`**: Sends a message and returns the complete response string. Blocks until generation is finished.
- **`chat_stream(message)`**: Returns a generator yielding response tokens as they arrive.
- **`stop_stream()`**: Interrupts an active streaming generation.
- **`with_memory(max_messages)`**: Enables history tracking. `max_messages` defines the context window size.
- **`clear_memory()`**: Resets the conversation history.