# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

"""
Custom Speaker Configuration Example

Demonstrates how to use a pre-configured Speaker instance with WaveGenerator.
Use this approach when you need:
- Specific USB speaker selection (USB_SPEAKER_2, etc.)
- Different audio format (S16_LE, etc.)
- Explicit device name ("plughw:CARD=Device,DEV=0")
"""

import time
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_peripherals.speaker import Speaker
from arduino.app_utils import App

# List available USB speakers
available_speakers = Speaker.list_usb_devices()
print(f"Available USB speakers: {available_speakers}")

# Create and configure a Speaker with specific parameters
# For optimal real-time synthesis, align periodsize with WaveGenerator block_duration
block_duration = 0.03  # Default WaveGenerator block duration
sample_rate = 16000
periodsize = int(sample_rate * block_duration)  # 480 frames @ 16kHz

speaker = Speaker(
    device=Speaker.USB_SPEAKER_1,  # or explicit device like "plughw:CARD=Device"
    sample_rate=sample_rate,
    channels=1,
    format="FLOAT_LE",
    periodsize=periodsize,  # Align with WaveGenerator blocks (eliminates glitches)
    queue_maxsize=10,  # Low latency for real-time audio
)

# Start the external Speaker manually
# WaveGenerator won't manage its lifecycle (ownership pattern)
speaker.start()

# Create WaveGenerator with the external speaker
wave_gen = WaveGenerator(
    sample_rate=sample_rate,
    speaker=speaker,  # Pass pre-configured speaker
    wave_type="sine",
    glide=0.02,
)

# Start the WaveGenerator (speaker already started above)
App.start_brick(wave_gen)


def play_sequence():
    """Play a simple frequency sequence."""
    frequencies = [261.63, 293.66, 329.63, 349.23, 392.00, 440.00, 493.88, 523.25]  # C4 to C5

    for freq in frequencies:
        print(f"Playing {freq:.2f} Hz")
        wave_gen.set_frequency(freq)
        wave_gen.set_amplitude(0.7)
        time.sleep(0.5)

    # Fade out
    wave_gen.set_amplitude(0.0)
    time.sleep(1)


print("Playing musical scale with external speaker...")
print("Press Ctrl+C to stop")

App.run(user_loop=play_sequence)

# Stop external Speaker manually (WaveGenerator doesn't manage external lifecycle)
speaker.stop()
print("Done")
