# remote_sensor API Reference

## Index

- Class `RemoteSensor`
- Class `RemoteSensorOpenError`
- Class `RemoteSensorConfigError`

---

## `RemoteSensor` class

```python
class RemoteSensor(port: int, timeout: int, certs_dir_path: str, use_tls: bool, secret: str | None, encrypt: bool, auto_reconnect: bool)
```

RemoteSensor implementation that hosts a WebSocket server.

This sensor acts as a WebSocket server that receives telemetry data from a
connected client. Only one client can be connected at a time.

Communication uses the BPP (Binary Peripheral Protocol) in three security modes:
- Security disabled (secret=None) - BPP with no authentication
- Authenticated (secret + encrypt=False) - BPP with HMAC-SHA256
- Authenticated + Encrypted (secret + encrypt=True) - BPP with ChaCha20-Poly1305

By default, all modes use BPP framing. When security is disabled (secret=None),
clients can opt out of BPP by connecting with the "raw=true" URL query parameter,
allowing them to send raw bytes directly without BPP wrapping. This parameter
is silently ignored when security is enabled.

When connecting, clients can specify a "client_name" parameter in the URL query string
to identify themselves. This name will be sanitized to allow only alphanumeric chars,
whitespace, hyphens, and underscores, and limit its length to 64 characters.

Each message is handed to the registered callback via the on_datapoint method.

### Parameters

- **port** (*int*): Port to bind the server to. Default: 8090.
- **timeout** (*int*): Connection timeout in seconds
- **certs_dir_path** (*str*): Path to the directory containing TLS certificates
- **use_tls** (*bool*): Enable TLS for secure connections. If True, 'encrypt' will
be ignored. Use this for transport-level security with clients that can
accept self-signed certificates or when supplying your own certificates.
- **secret** (*str | None*): Pre-shared secret key used for HMAC-SHA256
authentication, or to derive the ChaCha20-Poly1305 key when
encrypt is True. None disables security. Default: None.
- **encrypt** (*bool*): Enable ChaCha20-Poly1305 encryption. Requires a
non-None secret; raises RuntimeError otherwise. Default: False.
- **auto_reconnect** (*bool*): Enable automatic reconnection on failure

### Properties

#### `status: Literal['disconnected', 'connected', 'streaming', 'paused']`

Access: read-only

Read-only property for camera status.

#### `url: str`

Access: read-only

Return the WebSocket server address.

#### `security_mode: str`

Access: read-only

Return current security mode for logging/debugging.

### Methods

#### `start()`

Start the WebSocket server.

#### `on_datapoint(callback: Callable[[bytes], None])`

Register a callback function to be called when a datapoint is received.

The callback function will be called with a single argument: the binary
data received.

##### Parameters

- **callback** (*Callable[[bytes], None]*): A function that takes binary data
and returns None.

#### `is_started()`

Check if the sensor is started and running.

#### `on_status_changed(callback: Callable[[str, dict], None] | None)`

Registers or removes a callback to be triggered on camera lifecycle events.

When a camera status changes, the provided callback function will be invoked.
If None is provided, the callback will be removed.

##### Parameters

- **callback** (*Callable[[str, dict], None]*): A callback that will be called every time the
camera status changes with the new status and any associated data. The status names
depend on the actual camera implementation being used. Some common events are:
- 'connected': The camera has been reconnected.
- 'disconnected': The camera has been disconnected.
- 'streaming': The stream is streaming.
- 'paused': The stream has been paused and is temporarily unavailable.
- **callback** (*None*): To unregister the current callback, if any.

##### Examples

```python
def on_status(status: str, data: dict):
    print(f"Camera is now: {status}")
    print(f"Data: {data}")
    # Here you can add your code to react to the event

camera.on_status_changed(on_status)
```
#### `stop()`

Stop the WebSocket server.


---

## `RemoteSensorOpenError` class

```python
class RemoteSensorOpenError()
```

Exception raised when the remote sensor server cannot be opened.


---

## `RemoteSensorConfigError` class

```python
class RemoteSensorConfigError()
```

Exception raised when remote sensor configuration is invalid.

