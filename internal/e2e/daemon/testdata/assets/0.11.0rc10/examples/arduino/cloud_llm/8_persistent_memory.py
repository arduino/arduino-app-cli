# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Conversation with persistent memory"
# EXAMPLE_REQUIRES = "Requires a valid API key to a cloud LLM service and the dbstorage_sqlstore brick."

from arduino.app_bricks.cloud_llm import CloudLLM, SQLMessagePersistence
from arduino.app_bricks.dbstorage_sqlstore import SQLStore
from arduino.app_utils import App

# Share a single SQLStore instance across bricks that need it.
db = SQLStore("chat_history.db")
db.start()

llm = CloudLLM(
    api_key="YOUR_API_KEY",  # Replace with your actual API key
    system_prompt="You are a helpful assistant.",
).with_memory(
    max_messages=10,
    persistence=SQLMessagePersistence(sql_store=db, thread_id="demo-user"),
)


def ask_prompt():
    prompt = input("Enter your prompt (or 'exit' to quit, 'forget' to clear history): ")
    if prompt.lower() == "exit":
        raise StopIteration()
    if prompt.lower() == "forget":
        llm.clear_memory()
        print("Memory cleared for this thread.")
        return
    print(llm.chat(prompt))


App.run(ask_prompt)
