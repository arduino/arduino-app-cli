# Telegram Bot Brick

This Brick provides a user-friendly Python interface to create Telegram bots with synchronous API for creating interactive chatbots that can handle text messages, commands, and media files.

## Overview

The Telegram Bot Brick allows you to:

- Create Telegram bots with a simple synchronous API (no async/await needed)
- Handle commands, text messages, and media files (photos, audio, video, documents)
- Reply to users without manually tracking chat IDs using convenient `sender.reply*()` methods
- Schedule messages and manage user authorization via whitelist
- Automatically welcome users with built-in `/start` command and unblock detection

## Features

- **Simple synchronous API**: No async/await required in user code
- **Effortless replies**: Use `sender.reply()` instead of `bot.send_message(chat_id, ...)`
- **Media support**: Automatically download and process photos, audio, video, documents
- **Built-in welcome system**: Automatic `/start` command and unblock detection with smart cooldown
- **User authorization**: Whitelist specific Telegram user IDs to restrict bot access
- **Message scheduling**: Schedule messages to be sent after a delay
- **Contextual logging**: Track user interactions with automatic log prefixing
- **Robust error handling**: Automatic retries with configurable timeouts

## Prerequisites

- Telegram Bot Token from [@BotFather](https://t.me/botfather)
- Set `TELEGRAM_BOT_TOKEN` environment variable in Brick's settings

## Code Example and Usage

This example shows how to create a bot with command handlers and text echo functionality.

```python
from arduino.app_bricks.telegram_bot import TelegramBot, Sender, Message
from arduino.app_utils import App

# Initialize bot (reads token from TELEGRAM_BOT_TOKEN env var)
bot = TelegramBot()


def hello_command(sender: Sender, message: Message):
    """Handle /hello command with clean reply syntax."""
    sender.reply(f"👋 Hello {sender.first_name}! Welcome to the bot.")


def echo_text(sender: Sender, message: Message):
    """Echo back any text message the user sends."""
    sender.reply(f"You said: {message.text}")


# Register command handler with description (shows in Telegram menu)
bot.add_command("hello", hello_command, "Say hello")

# Handle all text messages (excluding commands)
bot.on_text(echo_text)

# Start the bot
App.run()
```

### Handling Photos

```python
from arduino.app_bricks.telegram_bot import TelegramBot, Sender, Message
from arduino.app_utils import App

bot = TelegramBot()


def process_photo(sender: Sender, message: Message, photo_bytes: bytes, filename: str, size: int):
    """Process received photos - photo data is automatically downloaded."""
    sender.reply(f"📸 Received '{filename}' ({size / 1024:.1f} KB)")
    
    # Process photo bytes here (e.g., with PIL, OpenCV, object detection)
    # ...
    
    # Reply with processed photo
    sender.reply_photo(photo_bytes, "Here's your photo!")


bot.on_photo(process_photo)

App.run()
```

## Configuration

The Brick is initialized with the following parameters:

| Parameter                | Type           | Default                          | Description                                                                                          |
| :----------------------- | :------------- | :------------------------------- | :--------------------------------------------------------------------------------------------------- |
| `token`                  | `str \| None`  | `os.getenv("TELEGRAM_BOT_TOKEN")` | Telegram bot API token. **Recommended:** set via environment variable.                               |
| `message_timeout`        | `int`          | `30`                             | Timeout in seconds for text message operations.                                                      |
| `media_timeout`          | `int`          | `60`                             | Timeout in seconds for media operations (photos, videos, etc.).                                       |
| `max_retries`            | `int`          | `3`                              | Maximum retry attempts for failed operations.                                                        |
| `auto_set_commands`      | `bool`         | `True`                           | Automatically sync command descriptions with Telegram UI.                                            |
| `enable_builtin_welcome` | `bool`         | `False`                           | Enable built-in `/start` command and welcome message on unblock.                                     |
| `whitelist_user_ids`     | `list[int] \| None` | `None`                      | Optional list of authorized user IDs. Only these users can interact with the bot.                    |

## Key Methods

### Registering Handlers

- **`add_command(command, callback, description="")`**: Register a command handler (e.g., `/start`)
- **`on_text(callback)`**: Handle all text messages (excluding commands)
- **`on_photo(callback)`**: Handle photo messages with automatic download
- **`on_audio(callback)`**: Handle audio messages with automatic download
- **`on_video(callback)`**: Handle video messages with automatic download
- **`on_document(callback)`**: Handle document messages with automatic download

### Sending Messages (Advanced)

For cases where you need to send messages outside of reply context:

- **`send_message(chat_id, text)`**: Send text message to specific chat
- **`send_photo(chat_id, photo_bytes, caption="")`**: Send photo to specific chat
- **`send_audio(chat_id, audio_bytes, caption="", filename="audio.mp3")`**: Send audio file
- **`send_video(chat_id, video_bytes, caption="", filename="video.mp4", supports_streaming=True)`**: Send video file
- **`send_document(chat_id, doc_bytes, filename, caption="")`**: Send document file

### Scheduling

- **`schedule_message(task_id, chat_id, message_text, delay_seconds)`**: Schedule a message to be sent after delay
- **`cancel_scheduled_message(task_id)`**: Cancel a previously scheduled message

## Reply Helpers (Recommended)

The `Sender` object passed to callbacks includes convenient reply methods:

- **`sender.reply(text)`**: Reply with text message
- **`sender.reply_photo(photo_bytes, caption="")`**: Reply with photo
- **`sender.reply_audio(audio_bytes, caption="", filename="audio.mp3")`**: Reply with audio
- **`sender.reply_video(video_bytes, caption="", filename="video.mp4")`**: Reply with video
- **`sender.reply_document(doc_bytes, filename, caption="")`**: Reply with document

**Why use reply methods?** They eliminate the need to manually track `chat_id`, making your callback code cleaner and more intuitive for chatbot logic.

## Media File Limits

Due to Telegram Bot API constraints (not hosting a local API server):

### Download Limits
- Photos: 10 MB maximum
- Audio/Video/Documents: 20 MB maximum

### Upload Limits
- Photos: 10 MB maximum
- Audio/Video files: 50 MB maximum
- Documents: 50 MB maximum

**Note**: All media is processed in RAM (no disk writes). For larger files, consider asking users to compress media or use external hosting services.