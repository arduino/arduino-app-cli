# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Register and handle commands"
# EXAMPLE_REQUIRES = "Requires TELEGRAM_BOT_TOKEN environment variable."

from arduino.app_bricks.telegram_bot import TelegramBot, Sender, Message
from arduino.app_utils import App

bot = TelegramBot()


def start_command(sender: Sender, message: Message):
    """Handle /start command."""
    sender.reply(f"👋 Welcome {sender.first_name}! I'm your Telegram bot.")


def hello_command(sender: Sender, message: Message):
    """Handle /hello command."""
    sender.reply(f"Hello {sender.first_name}! How can I help you today?")


bot.add_command("start", start_command, "Start the bot")
bot.add_command("hello", hello_command, "Say hello")

App.run()
