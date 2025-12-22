# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

"""
Envelope Control Example

Demonstrates amplitude envelope control with different
attack and release times for various sonic effects.
"""

import time
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_utils import App

wave_gen = WaveGenerator(wave_type="sine")
App.start_brick(wave_gen)

wave_gen.set_frequency(440.0)
wave_gen.set_volume(80)


def envelope_demo():
    """Demonstrate different envelope settings."""

    # Fast attack, fast release (percussive)
    print("1. Percussive (fast attack/release)...")
    wave_gen.set_envelope_params(attack=0.001, release=0.01, glide=0.0)
    wave_gen.set_amplitude(0.8)
    time.sleep(0.5)
    wave_gen.set_amplitude(0.0)
    time.sleep(1)

    # Slow attack, fast release (pad-like)
    print("2. Pad-like (slow attack, fast release)...")
    wave_gen.set_envelope_params(attack=0.2, release=0.05, glide=0.0)
    wave_gen.set_amplitude(0.8)
    time.sleep(1)
    wave_gen.set_amplitude(0.0)
    time.sleep(1)

    # Fast attack, slow release (sustained)
    print("3. Sustained (fast attack, slow release)...")
    wave_gen.set_envelope_params(attack=0.01, release=0.3, glide=0.0)
    wave_gen.set_amplitude(0.8)
    time.sleep(0.5)
    wave_gen.set_amplitude(0.0)
    time.sleep(1.5)

    # Medium attack and release (balanced)
    print("4. Balanced (medium attack/release)...")
    wave_gen.set_envelope_params(attack=0.05, release=0.05, glide=0.0)
    wave_gen.set_amplitude(0.8)
    time.sleep(0.8)
    wave_gen.set_amplitude(0.0)
    time.sleep(2)


print("Envelope Control Demonstration")
print("Listen to different attack/release characteristics")
print("Press Ctrl+C to stop")

App.run(user_loop=envelope_demo)
