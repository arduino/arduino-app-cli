# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

"""
Basic Wave Generator Example

Generates a simple 440Hz sine wave (A4 note) and demonstrates
basic frequency and amplitude control.
"""

from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_utils import App

# Create wave generator with default settings
wave_gen = WaveGenerator(
    sample_rate=16000,
    wave_type="sine",
    glide=0.02,  # 20ms smooth frequency transitions
)

# Start the generator
App.start_brick(wave_gen)

# Set initial frequency and amplitude
wave_gen.set_frequency(440.0)  # A4 note (440 Hz)
wave_gen.set_amplitude(0.7)  # 70% amplitude
wave_gen.set_volume(80)  # 80% hardware volume

print("Playing 440Hz sine wave (A4 note)")
print("Press Ctrl+C to stop")

# Keep the application running
App.run()
