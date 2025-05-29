import serial
import time
import msgpack
from io import BytesIO

REQUEST = 0
RESPONSE = 1
NOTIFY = 2

class SerialClient:
    def __init__(self, port, baudrate=115200):
        self.ser = serial.Serial(port, baudrate, timeout=0.1)
        self.msg_id = 0

    def call(self, method, *args):
        request = [REQUEST, self.msg_id, method, [*args]]
        self.ser.write(msgpack.packb(request))

        data = None
        while not data:
            data = self.ser.read(1024)

        unpacker = msgpack.Unpacker(BytesIO(data))
        for message in unpacker:
            print(message)

    def notify(self, method, *args):
        request = [NOTIFY, method, [*args]]
        self.ser.write(msgpack.packb(request))
