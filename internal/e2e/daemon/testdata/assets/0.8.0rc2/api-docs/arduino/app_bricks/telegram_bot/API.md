# telegram_bot API Reference

## Index

- Class `TelegramBot`
- Class `Sender`
- Class `Message`

---

## `TelegramBot` class

```python
class TelegramBot(token: Optional[str], message_timeout: int, media_timeout: int, max_retries: int, auto_set_commands: bool, enable_builtin_welcome: bool, whitelist_user_ids: Optional[list[int]])
```

A brick to manage Telegram Bot interactions with synchronous API.

Provides a user-friendly interface for creating Telegram bots using synchronous
methods while handling async operations internally. Includes automatic retries,
configurable timeouts, and built-in authorization via user ID whitelist.

### Parameters

- **token**: Telegram bot API token. Reads from TELEGRAM_BOT_TOKEN environment
variable if not provided.
- **message_timeout**: Timeout in seconds for text messages, defaults to 30.
- **media_timeout**: Timeout in seconds for media operations, defaults to 60.
- **max_retries**: Maximum retry attempts for failed operations, defaults to 3.
- **auto_set_commands**: Automatically sync command menu with Telegram,
defaults to True.
- **enable_builtin_welcome**: Automatically register /start command and
my_chat_member handler to welcome users. Shows user_id, chat_id,
and first_name. Disabled if user registers custom /start handler.
Defaults to False.
- **whitelist_user_ids**: Optional list of authorized Telegram user IDs.
If provided, only these users can interact with the bot.
Use @userinfobot on Telegram to get your user ID.

### Raises

- **ValueError**: If token not provided and TELEGRAM_BOT_TOKEN not set.

### Methods

#### `add_command(command: str, callback: Callable[[Sender, Message], None], description: str)`

Register a command handler.

##### Parameters

- **command**: Command name without leading '/', e.g., 'start'.
- **callback**: Function to call when command is received. Receives
Sender and Message objects.
- **description**: Optional description shown in Telegram's command menu.
If provided and auto_set_commands is True, appears when user types '/'.

#### `on_text(callback: Callable[[Sender, Message], None])`

Register a handler for all non-command text messages.

##### Parameters

- **callback**: Function to call for each text message. Receives
Sender and Message objects. Does not trigger for commands.

#### `on_photo(callback: Callable[[Sender, Message, bytes, str, int], None])`

Register a handler for photo messages with automatic download.

##### Parameters

- **callback**: Function to call when photo is received. Receives:
- sender: Sender information
- message: Message metadata
- photo_bytes: Downloaded photo data
- filename: Fixed name 'photo.jpg'
- size: Photo size in bytes

#### `on_audio(callback: Callable[[Sender, Message, bytes, str, int], None])`

Register a handler for audio messages with size-checked download.

##### Parameters

- **callback**: Function to call when audio is received. Receives:
- sender: Sender information
- message: Message metadata
- audio_bytes: Downloaded audio data
- filename: Original filename or 'audio.mp3'
- size: Audio size in bytes

#### `on_video(callback: Callable[[Sender, Message, bytes, str, int], None])`

Register a handler for video messages with size-checked download.

##### Parameters

- **callback**: Function to call when video is received. Receives:
- sender: Sender information
- message: Message metadata
- video_bytes: Downloaded video data
- filename: Original filename or 'video.mp4'
- size: Video size in bytes

#### `on_document(callback: Callable[[Sender, Message, bytes, str, int], None])`

Register a handler for document messages with size-checked download.

##### Parameters

- **callback**: Function to call when document is received. Receives:
- sender: Sender information
- message: Message metadata
- document_bytes: Downloaded document data
- filename: Original filename or 'document'
- size: Document size in bytes

#### `send_message(chat_id: int, message_text: str)`

Send a text message with automatic retry.

##### Parameters

- **chat_id**: Telegram chat ID to send the message to.
- **message_text**: Text content of the message.

##### Returns

-: True if message was sent successfully, False otherwise.

#### `send_photo(chat_id: int, photo_bytes: bytes, caption: str)`

Send a photo to a chat.

##### Parameters

- **chat_id**: Telegram chat ID.
- **photo_bytes**: Photo data as bytes.
- **caption**: Optional caption text.

##### Returns

-: True if successful, False otherwise.

#### `send_audio(chat_id: int, audio_bytes: bytes, caption: str, filename: str)`

Send an audio file to a chat.

##### Parameters

- **chat_id**: Telegram chat ID.
- **audio_bytes**: Audio data as bytes.
- **caption**: Optional caption text.
- **filename**: Filename with extension, defaults to 'audio.mp3'.
Extension helps Telegram determine MIME type.
Supported: .mp3, .m4a, .ogg, etc.

##### Returns

-: True if successful, False otherwise.

#### `send_video(chat_id: int, video_bytes: bytes, caption: str, filename: str, supports_streaming: bool)`

Send a video to a chat.

##### Parameters

- **chat_id**: Telegram chat ID.
- **video_bytes**: Video data as bytes.
- **caption**: Optional caption text.
- **filename**: Filename with extension, defaults to 'video.mp4'.
Extension helps Telegram determine MIME type. Use .mp4 for best compatibility.
- **supports_streaming**: Enable progressive download for MP4/H.264 videos,
allowing playback before download completes. Only effective for
MPEG4 format. Defaults to True.

##### Returns

-: True if successful, False otherwise.

#### `send_document(chat_id: int, document_bytes: bytes, filename: str, caption: str)`

Send a document to a chat.

##### Parameters

- **chat_id**: Telegram chat ID.
- **document_bytes**: Document data as bytes.
- **filename**: Document filename. Include extension for proper MIME type detection.
- **caption**: Optional caption text.

##### Returns

-: True if successful, False otherwise.

#### `schedule_message(chat_id: int, message_text: str, interval_seconds: int, task_id: Optional[str])`

Schedule a recurring message at regular intervals.

##### Parameters

- **chat_id**: Telegram chat ID to send messages to.
- **message_text**: Text content of the scheduled message.
- **interval_seconds**: Time interval in seconds between messages.
- **task_id**: Optional unique identifier for this task.
If not provided, one is generated automatically.

##### Returns

-: Task ID string that can be used to cancel the scheduled message.
Returns empty string if bot is not initialized.

#### `cancel_scheduled_message(task_id: str)`

Cancel a scheduled message task.

##### Parameters

- **task_id**: ID of the task to cancel, as returned by schedule_message().

##### Returns

-: True if task was found and cancelled, False otherwise.

#### `start()`

Start the Telegram bot in a background thread.

Initializes the bot and starts polling for updates in a separate thread,
allowing the main application to continue running. Waits for successful
initialization before returning.

##### Raises

- **RuntimeError**: If bot fails to initialize within 30 seconds timeout.

#### `stop()`

Stop the Telegram bot gracefully.

Stops polling, cancels all scheduled messages, shuts down the application,
and waits for the background thread to terminate.


---

## `Sender` class

```python
class Sender(chat_id: int, user_id: int, first_name: str, last_name: Optional[str], username: Optional[str])
```

Represents the sender of a Telegram message.

Contains user identification and provides convenient methods for replying
to messages without manually specifying the chat ID.

### Attributes

- **chat_id**: Telegram chat ID for sending responses.
- **user_id**: Unique Telegram user identifier.
- **first_name**: User's first name.
- **last_name**: User's last name, None if not set.
- **username**: User's Telegram username (without @), None if not set.

### Methods

#### `reply(text: str)`

Send a text reply to the sender.

##### Parameters

- **text**: Text content to send.

##### Returns

-: True if message was sent successfully, False otherwise.

#### `reply_photo(photo_bytes: bytes, caption: str)`

Send a photo reply to the sender.

##### Parameters

- **photo_bytes**: Photo data as bytes.
- **caption**: Optional text caption for the photo.

##### Returns

-: True if photo was sent successfully, False otherwise.

#### `reply_audio(audio_bytes: bytes, caption: str, filename: str)`

Send an audio file reply to the sender.

##### Parameters

- **audio_bytes**: Audio file data as bytes.
- **caption**: Optional text caption for the audio.
- **filename**: Filename with extension, defaults to "audio.mp3".

##### Returns

-: True if audio was sent successfully, False otherwise.

#### `reply_video(video_bytes: bytes, caption: str, filename: str, supports_streaming: bool)`

Send a video file reply to the sender.

##### Parameters

- **video_bytes**: Video file data as bytes.
- **caption**: Optional text caption for the video.
- **filename**: Filename with extension, defaults to "video.mp4".
- **supports_streaming**: Enable progressive playback for MP4/H.264 videos.
Allows playback before full download. Ignored for other formats.

##### Returns

-: True if video was sent successfully, False otherwise.

#### `reply_document(document_bytes: bytes, filename: str, caption: str)`

Send a document file reply to the sender.

##### Parameters

- **document_bytes**: Document file data as bytes.
- **filename**: Filename for the document.
- **caption**: Optional text caption for the document.

##### Returns

-: True if document was sent successfully, False otherwise.


---

## `Message` class

```python
class Message(message_id: int, text: Optional[str], caption: Optional[str])
```

Represents a Telegram message content and metadata.

### Attributes

- **message_id**: Unique message identifier.
- **text**: Text content, None if message has no text.
- **caption**: Media caption text, None if no caption present.

