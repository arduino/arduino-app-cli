# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

"""
Frequency Sweep Example

Demonstrates smooth frequency transitions (glide/portamento effect)
by sweeping through different frequency ranges.
"""

import time
from arduino.app_bricks.wave_generator import WaveGenerator
from arduino.app_utils import App

wave_gen = WaveGenerator(
    wave_type="sine",
    glide=0.05,  # 50ms glide for noticeable portamento
)

App.start_brick(wave_gen)
wave_gen.set_amplitude(0.7)


def frequency_sweep():
    """Sweep through frequency ranges."""

    # Low to high sweep
    print("Sweeping low to high (220Hz → 880Hz)...")
    for freq in range(220, 881, 20):
        wave_gen.set_frequency(float(freq))
        time.sleep(0.1)

    time.sleep(0.5)

    # High to low sweep
    print("Sweeping high to low (880Hz → 220Hz)...")
    for freq in range(880, 219, -20):
        wave_gen.set_frequency(float(freq))
        time.sleep(0.1)
    # Fade out
    print("Fading out...")
    wave_gen.set_amplitude(0.0)
    time.sleep(2)


print("Frequency sweep demonstration")
print("Listen for smooth glide between frequencies")
print("Press Ctrl+C to stop")

App.run(user_loop=frequency_sweep)
