# mqtt API Reference

## Index

- Function `generate_client_id`
- Class `MQTTSink`
- Class `MQTTSource`

---

## `generate_client_id` function

```python
def generate_client_id(name: str)
```

Generate a unique client ID for MQTT clients.

### Parameters

- **name** (*str*): The base name for the client ID.

### Returns

- (*str*): A unique client ID combining the base name and a UUID.


---

## `MQTTSink` class

```python
class MQTTSink(broker_address: str, broker_port: int, topic: str, username: str, password: str, client_id: str)
```

MQTT Sink for publishing messages to a specified topic.

### Parameters

- **broker_address** (*str*): The address of the MQTT broker.
- **broker_port** (*int*): The port of the MQTT broker.
- **topic** (*str*): The topic to publish messages to.
- **username** (*str*): The username for MQTT authentication.
- **password** (*str*): The password for MQTT authentication.
- **client_id** (*str*) (optional): A unique client ID for the MQTT client.

### Methods

#### `start()`

Start the MQTT client and connect to the broker.

#### `stop()`

Stop the MQTT client and disconnect from the broker.

#### `write(message: str | dict)`

Publish a message to the MQTT topic.

##### Parameters

- **message** (*str|dict*): The message to publish. Can be a string or a dictionary.

##### Returns

- (*mqtt.MQTTMessageInfo*): The result of the publish operation.

#### `consume(item)`

Process an item and publish it to the MQTT topic.

##### Parameters

- **item**: The item to be processed. Can be a string or a dictionary.

##### Returns

- (*None*): If the item is None or if the item type is invalid.


---

## `MQTTSource` class

```python
class MQTTSource(broker_address: str, broker_port: int, topic: str, username: str, password: str, client_id: str)
```

MQTT Source for subscribing to a specified topic and receiving messages.

### Parameters

- **broker_address** (*str*): The address of the MQTT broker.
- **broker_port** (*int*): The port of the MQTT broker.
- **topic** (*str*): The topic to subscribe to for receiving messages.
- **username** (*str*): The username for MQTT authentication.
- **password** (*str*): The password for MQTT authentication.
- **client_id** (*str*) (optional), default="Arduino_MQTTSource": A unique client ID for the MQTT client. Defaults to "Arduino_MQTTSource".

### Methods

#### `stop()`

Stop the MQTT client and clear the message queue.

#### `wait_for_message()`

Wait for a message to be received from the MQTT topic.

This method blocks until a message is available in the queue.

##### Returns

- (*str*): The received message payload.

#### `produce()`

Produce messages by waiting for them to be received from the MQTT topic.

This method blocks until a message is available in the queue and returns it.

##### Returns

- (*str*): The received message payload.

