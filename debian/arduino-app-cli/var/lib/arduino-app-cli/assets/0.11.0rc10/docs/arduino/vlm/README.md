# Vision Language Model (VLM) Brick

The Vision Language Model (VLM) Brick provides a simple Python® interface for asking a locally hosted multimodal AI model questions about images. It lets Arduino® App Lab applications send text prompts together with image file paths or image bytes, then receive generated text from the model running on the board.

## Overview

The VLM Brick is designed for applications that need visual understanding without sending camera frames to the cloud. It wraps the local model runner behind the same chat-style API used by the LLM Bricks, while adding image input support through the `images` parameter.

Use this Brick when your application needs to describe a camera frame, inspect a scene, extract visual details, or combine an image with a natural-language instruction. For example, the Smart Mirror example captures a USB camera frame, sends it to `VisionLanguageModel`, and displays a short styling response in a web UI.

The Brick uses the model configured for `arduino:vlm` in Arduino App Lab.

## Features

- **Local visual AI**: Sends prompts to a VLM running on the board through the local model service.
- **Text plus image prompts**: Accepts a text message and one or more images as local file paths or raw image bytes.
- **Synchronous responses**: Uses `chat()` when the application needs the full answer before continuing.
- **Streaming responses**: Uses `chat_stream()` when the application should display generated text as it arrives.
- **Conversation memory**: Keeps recent chat history with `with_memory()` and can persist it across restarts with `persistence=True`.
- **Configurable generation**: Supports system prompts, temperature, token limits, timeouts, and model overrides.
- **Advanced access**: Exposes the underlying LangChain chat model through `get_client()` for custom integrations.

## Prerequisites

- A supported board with a local VLM model runner available. The current Brick configuration supports `ventunoq`.
- A compatible VLM model downloaded and configured in Arduino App Lab.
- The `arduino:vlm` Brick added to the application from App Lab.
- Image input as a valid file path or bytes. Camera frames should be encoded as JPEG bytes before calling the Brick.

**Note:** The VLM runs locally through the board model service, so cloud inference and cloud API keys are not required for normal use.

## Code example and usage

### Analyze an Image File

This example sends a local image file and waits for the complete model response.

```python
from arduino.app_bricks.vlm import VisionLanguageModel

vlm = VisionLanguageModel(
    system_prompt="You are a concise visual assistant.",
    temperature=0.4,
    max_tokens=120,
)

response = vlm.chat(
    message="Describe the main object in this image.",
    images=["chair.jpg"],
)

print(response)
```

### Stream a Response

Use `chat_stream()` when a web UI or terminal interface should show text as the model generates it.

```python
from arduino.app_bricks.vlm import VisionLanguageModel

vlm = VisionLanguageModel()

for chunk in vlm.chat_stream(
    message="Describe the image in one short paragraph.",
    images=["chair.jpg"],
):
    print(chunk, end="", flush=True)
```

### Analyze a Camera Frame

The Smart Mirror example uses this pattern: it captures the latest camera frame as JPEG bytes and sends the bytes directly to the VLM.

```python
from arduino.app_bricks.vlm import VisionLanguageModel

vlm = VisionLanguageModel(
    system_prompt="You are a helpful visual assistant.",
    temperature=0.4,
    max_tokens=120,
)

def analyze_frame(frame_bytes: bytes) -> str:
    return vlm.chat(
        message="Describe the most important visual details in this frame.",
        images=[frame_bytes],
    ).strip()
```

### Enable Conversation Memory

Use `with_memory()` when follow-up prompts should keep recent visual context across calls. Pass `persistence=True` to retain memory across restarts — this uses the `SQLStore` brick internally, so no extra setup is required.

```python
from arduino.app_bricks.vlm import VisionLanguageModel

vlm = VisionLanguageModel(
    system_prompt="You remember relevant visual details.",
).with_memory(
    max_messages=10,
    persistence=True,
)

print(vlm.chat("Remember what is in this image.", images=["chair.jpg"]))
print(vlm.chat("What object did I show you earlier?"))
```

## Configuration

The Brick is initialized with the following parameters:

| Parameter | Type | Default | Description |
| :-- | :-- | :-- | :-- |
| `api_key` | `str` | `os.getenv("LOCAL_LLM_API_KEY", "api_key")` | API key value passed to the local OpenAI-compatible model service. The default placeholder is enough for normal local use. |
| `model` | `str` | App Lab configured model | Local model identifier configured for `arduino:vlm` in App Lab. |
| `system_prompt` | `str` | `""` | System-level instruction that defines the assistant behavior. |
| `temperature` | `float` \| `None` | `0.7` | Controls randomness. Lower values are more deterministic; higher values are more varied. |
| `max_tokens` | `int` | `512` | Maximum number of tokens to generate in the response. |
| `timeout` | `int` \| `None` | `None` | Maximum time in seconds to wait for a response. |
| `tools` | `list[Callable]` | `None` | Optional LangChain-compatible tool functions available to the model. |
| `**kwargs` | `dict` | `{}` | Additional keyword arguments passed to the underlying model constructor. |

The Brick configuration declares that `arduino:vlm` requires a local model service and a compatible model:

```yaml
id: arduino:vlm
name: Vision Language Model (VLM)
requires_services: ["arduino:genie"]
model: genie:qwen2_5_vl_7b_instruct
supported_boards: ["ventunoq"]
```

## Methods

- **`chat(message, images=None)`**: Sends a prompt and optional images, then returns the complete generated response as a string.
- **`chat_stream(message, images=None)`**: Sends a prompt and optional images, then yields generated text chunks as they arrive.
- **`stop_stream()`**: Requests cancellation of the active streaming response.
- **`with_memory(max_messages=0, persistence=None)`**: Enables conversational memory for the instance. `persistence=True` enables persistence with a default database/thread; pass a `MessagePersistence` for full control. Pass `max_messages=0` to disable history.
- **`clear_memory()`**: Clears the active conversation history.
- **`get_client()`**: Returns the underlying LangChain `BaseChatModel` instance.

## Image Inputs

The `images` argument accepts a list containing:

- File paths, such as `"chair.jpg"`.
- Raw image bytes, such as a JPEG frame captured from a camera.

When an image path is used, the file must exist in the application runtime environment. When bytes are used, make sure they represent an encoded image format that the model can interpret. For camera applications, convert frames to JPEG bytes before passing them to the Brick.

## Example Application

The `app-bricks-examples` repository includes a Smart Mirror example for Arduino® VENTUNO™ Q that uses the VLM Brick with a web UI and USB camera:

```python
result = vlm.chat(
    message=prompt.build_user_prompt(USER_PROMPT_TEMPLATE),
    images=[frame],
).strip()
```

In that example, the application continuously captures camera frames, keeps the latest frame in memory, and sends one frame to the VLM when the user starts a scan from the browser interface.

## Troubleshooting

### Model not found

**Fix:** Verify that the selected VLM model is downloaded and available in App Lab. If you override the model, make sure the model identifier matches a model exposed by the local `genie` service.

### Empty or generic responses

**Fix:** Use a more specific prompt, lower the temperature for more consistent output, and make sure the image is clear and well lit. For camera frames, verify that the bytes are encoded image data, not raw pixel data.

### Image file not found

**Fix:** Use a path that exists inside the application container, or pass image bytes instead of a file path.

### Response generation fails with a memory error

**Fix:** Reduce `max_tokens`, close other running applications, and restart the app. The VLM can require significant memory during model loading and inference.
