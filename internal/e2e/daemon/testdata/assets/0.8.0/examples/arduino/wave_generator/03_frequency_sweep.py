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
    glide=0.05,  # 50ms glide for noticeable portamento
)
wave_gen.amplitude = 0.7


def frequency_sweep():
    """Sweep through frequency ranges."""
    # Low to high sweep
    for freq in range(220, 881, 20):
        wave_gen.frequency = float(freq)
        time.sleep(0.1)

    time.sleep(0.5)

    # High to low sweep
    for freq in range(880, 219, -20):
        wave_gen.frequency = float(freq)
        time.sleep(0.1)

    wave_gen.amplitude = 0.0  # Fade out
    time.sleep(2)


App.run(user_loop=frequency_sweep)
