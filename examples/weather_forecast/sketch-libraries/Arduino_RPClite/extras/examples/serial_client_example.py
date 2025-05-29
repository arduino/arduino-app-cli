from serial_client import SerialClient

PORT = '/dev/ttySTM0'

client = SerialClient(port=PORT, baudrate=115200)


client.call("add", 10)       # not enough args

client.call("add", 15, 7)   # just enough args

client.call("add", 1, 2, 3) # too many args

client.call("greet")

client.notify("add", 5, 9)

client.notify("greet")