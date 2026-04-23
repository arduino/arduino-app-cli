# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Echo text messages"
# EXAMPLE_REQUIRES = "Requires TELEGRAM_BOT_TOKEN environment variable."

from arduino.app_bricks.telegram_bot import TelegramBot, Sender, Message
from arduino.app_utils import App

bot = TelegramBot()


def echo_text(sender: Sender, message: Message):
    """Echo back the received text message."""
    sender.reply(f"You said: {message.text}")


bot.on_text(echo_text)

App.run()
