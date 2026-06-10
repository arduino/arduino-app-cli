# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Local VLM chat with persistent memory"
# EXAMPLE_REQUIRES = "Local VLM models available and the dbstorage_sqlstore brick."

from arduino.app_bricks.cloud_llm import SQLMessagePersistence
from arduino.app_bricks.dbstorage_sqlstore import SQLStore
from arduino.app_bricks.vlm import VisionLanguageModel
from arduino.app_utils import App

db = SQLStore("vlm_persistent_demo.db")
db.start()

vlm = VisionLanguageModel(
    system_prompt="You are a helpful visual assistant.",
).with_memory(
    max_messages=10,
    persistence=SQLMessagePersistence(sql_store=db, thread_id="vlm-demo-conversation"),
)


def ask_prompt():
    prompt = input("Enter your prompt (or 'exit' to quit, 'forget' to clear history, 'image' to include chair.jpg): ")
    if prompt.lower() == "exit":
        raise StopIteration()
    if prompt.lower() == "forget":
        vlm.clear_memory()
        print("Memory cleared for this thread.")
        return

    images = ["chair.jpg"] if prompt.lower() == "image" else None
    message = "Describe the image and remember relevant visual details." if images else prompt
    print(vlm.chat(message, images=images))


App.run(ask_prompt)
