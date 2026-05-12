# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Echo audio messages"
# EXAMPLE_REQUIRES = "Requires TELEGRAM_BOT_TOKEN environment variable."

from arduino.app_bricks.telegram_bot import TelegramBot, Sender, Message
from arduino.app_utils import App

bot = TelegramBot()


def echo_audio(sender: Sender, message: Message, audio_bytes: bytes, filename: str, size: int):
    """Echo back the received audio file."""
    caption = f"🎵 Audio received! File: {filename}, Size: {size / 1024:.1f} KB"
    if message.caption:
        caption += f"\nOriginal caption: {message.caption}"

    sender.reply_audio(audio_bytes, caption, filename)


bot.on_audio(echo_audio)

App.run()
