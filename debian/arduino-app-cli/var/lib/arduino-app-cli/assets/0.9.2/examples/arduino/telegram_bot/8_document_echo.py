# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Echo document messages"
# EXAMPLE_REQUIRES = "Requires TELEGRAM_BOT_TOKEN environment variable."

from arduino.app_bricks.telegram_bot import TelegramBot, Sender, Message
from arduino.app_utils import App

bot = TelegramBot()


def echo_document(sender: Sender, message: Message, document_bytes: bytes, filename: str, size: int):
    """Echo back the received document file."""
    caption = f"📄 Document received! File: {filename}, Size: {size / 1024:.1f} KB"
    if message.caption:
        caption += f"\nOriginal caption: {message.caption}"

    sender.reply_document(document_bytes, filename, caption)


bot.on_document(echo_document)

App.run()
