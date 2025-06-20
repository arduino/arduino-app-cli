# MQTT Connector brick

An MQTT connector designed to both publish messages to specified topics and subscribe to incoming messages from other clients, enabling seamless two-way communication.

## Code example and usage

Connector can be used as publisher or subscriber to exchange messages over a MQTT server.

### Message publishing

Sample code for message publishing

```python
from arduino.app_bricks.mqtt import MQTTSink

mqtt_sink = MQTTSink(
    broker_address="192.168.1.18", broker_port=1883, topic="my_topic",
    username="admin", password="password")

# Send a message to the topic
msqg = {
    "key1": "value1",
    "key2": "value2"
} 
mqtt_sink.write(msqg)

# Close the MQTT sink when done
mqtt_sink.stop()
```

### Topic subscriber

Sample code for subscribing to a topic and receive messages

```python
from arduino.app_bricks.mqtt import MQTTSource

mqtt_src = MQTTSource(
    broker_address="192.168.1.18", broker_port=1883, topic="my_topic",
    username="admin", password="password")

while True:
    msg = mqtt_src.wait_for_message() # Blocking call, waits for a message
    if msg is None:
        continue
    print(f"Received message: {msg}")
```
