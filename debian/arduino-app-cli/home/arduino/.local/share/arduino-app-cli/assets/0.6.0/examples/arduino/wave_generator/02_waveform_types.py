# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

"""
Waveform Comparison Example

Cycles through different waveform types to hear the difference
between sine, square, sawtooth, and triangle waves.
"""

import time
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_utils import App

wave_gen = WaveGenerator(sample_rate=16000, glide=0.02)
App.start_brick(wave_gen)

# Set constant frequency and amplitude
wave_gen.set_frequency(440.0)
wave_gen.set_amplitude(0.6)

waveforms = ["sine", "square", "sawtooth", "triangle"]


def cycle_waveforms():
    """Cycle through different waveform types."""
    for wave_type in waveforms:
        print(f"Playing {wave_type} wave...")
        wave_gen.set_wave_type(wave_type)
        time.sleep(3)
    # Silence
    wave_gen.set_amplitude(0.0)
    time.sleep(2)


print("Cycling through waveforms:")
print("sine → square → sawtooth → triangle")
print("Press Ctrl+C to stop")

App.run(user_loop=cycle_waveforms)
