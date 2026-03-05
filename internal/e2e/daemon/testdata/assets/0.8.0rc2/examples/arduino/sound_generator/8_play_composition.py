# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME: Play a MusicComposition
from arduino.app_bricks.sound_generator import SoundGenerator, SoundEffect, MusicComposition
from arduino.app_utils import App

# Define a composition as a data structure
comp = MusicComposition(
    composition=[
        [("C4", 1 / 16), ("E4", 1 / 16)],
        [("G4", 1 / 16)],
        [("A4", 1 / 16), ("C5", 1 / 16)],
        [],
        [("G4", 1 / 16)],
        [("F4", 1 / 16)],
        [("E4", 1 / 16), ("G4", 1 / 16)],
        [],
        [("D4", 1 / 16)],
        [("C4", 1 / 16)],
        [],
        [],
    ],
    bpm=140,
    waveform="square",
    volume=0.8,
    effects=[SoundEffect.adsr(), SoundEffect.tremolo(depth=0.4, rate=4.0)],
)

player = SoundGenerator()
player.play_composition(comp, block=True)

App.run()
