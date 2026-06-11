# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Local LLM chat with persistent memory"
# EXAMPLE_REQUIRES = "Local LLM models available and the dbstorage_sqlstore brick."

from arduino.app_bricks.cloud_llm import SQLMessagePersistence
from arduino.app_bricks.dbstorage_sqlstore import SQLStore
from arduino.app_bricks.llm import LargeLanguageModel
from arduino.app_utils import App

db = SQLStore("llm_persistent_demo.db")
db.start()

llm = LargeLanguageModel(
    system_prompt="You are a helpful assistant.",
).with_memory(
    max_messages=10,
    persistence=SQLMessagePersistence(sql_store=db, thread_id="llm-demo-conversation"),
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
