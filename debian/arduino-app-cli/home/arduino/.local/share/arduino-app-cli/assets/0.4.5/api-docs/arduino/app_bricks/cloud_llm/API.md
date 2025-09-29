# cloud_llm API Reference

## Index

- Class `CloudLLM`
- Class `CloudModel`

---

## `CloudLLM` class

```python
class CloudLLM(api_key: str, model: Union[str, CloudModel], system_prompt: str, temperature: Optional[float], timeout: int)
```

A simplified, opinionated wrapper for common LangChain conversational patterns.

This class provides a single interface to manage stateless chat and chat with memory.

### Parameters

- **api_key**: The API key for the LLM service.
- **model**: The model identifier as per LangChain specification (e.g., "anthropic:claude-3-sonnet-20240229")
or by using a CloudModels enum (e.g. CloudModels.OPENAI_GPT).
- **system_prompt**: The global system-level instruction for the AI.

### Raises

- **ValueError**: If the API key is missing.

### Methods

#### `with_memory(max_messages: int)`

Enables conversational memory for this instance.

This allows the chatbot to remember previous user and AI messages.
Calling this modifies the instance to be stateful.

##### Parameters

- **max_messages**: The total number of past messages (user + AI) to
keep in the conversation window. Set to 0 to disable memory.

##### Returns

- (*self*): The current CloudLLM instance for method chaining.

#### `chat(message: str)`

Sends a single message to the AI and gets a complete response synchronously.

This is the primary way to interact. It automatically handles memory
based on how the instance was configured.

##### Parameters

- **message**: The user's message.

##### Returns

-: The AI's complete response as a string.

##### Raises

- **RuntimeError**: If the chat model is not initialized or if text generation fails.

#### `chat_stream(message: str)`

Sends a single message to the AI and streams the response as a synchronous generator.

Use this to get tokens as they are generated, perfect for a streaming UI.

##### Parameters

- **message**: The user's message.

##### Returns

- (*str*): Chunks of the AI's response as they become available.

##### Raises

- **RuntimeError**: If the chat model is not initialized or if text generation fails.
- **AlreadyGenerating**: If the chat model is already streaming a response.

#### `stop_stream()`

Signals the LLM to stop generating a response.

#### `clear_memory()`

Clears the conversational memory.

This only has an effect if with_memory() has been called.


---

## `CloudModel` class

```python
class CloudModel()
```

