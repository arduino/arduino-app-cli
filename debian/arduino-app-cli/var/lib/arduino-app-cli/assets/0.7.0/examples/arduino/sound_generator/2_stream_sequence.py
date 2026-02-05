# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME: Stream a sequence of notes over websocket via WebUI
import time
from arduino.app_utils import *
from arduino.app_bricks.web_ui import WebUI
from arduino.app_bricks.sound_generator import SoundGeneratorStreamer, SoundEffect

ui = WebUI()

player = SoundGeneratorStreamer(master_volume=1.0, wave_form="square", bpm=120, sound_effects=[SoundEffect.adsr()])

tune_sequence = [
    ("E5", 0.125),
    ("E5", 0.125),
    ("REST", 0.125),
    ("E5", 0.125),
    ("REST", 0.125),
    ("C5", 0.125),
    ("E5", 0.125),
    ("REST", 0.125),
    ("G5", 0.25),
    ("REST", 0.25),
    ("G4", 0.25),
    ("REST", 0.25),
    ("C5", 0.25),
    ("REST", 0.125),
    ("G4", 0.25),
    ("REST", 0.125),
    ("E4", 0.25),
    ("REST", 0.125),
    ("A4", 0.25),
    ("B4", 0.25),
    ("Bb4", 0.125),
    ("A4", 0.25),
    ("G4", 0.125),
    ("E5", 0.125),
    ("G5", 0.125),
    ("A5", 0.25),
    ("F5", 0.125),
    ("G5", 0.125),
    ("REST", 0.125),
    ("E5", 0.25),
    ("C5", 0.125),
    ("D5", 0.125),
    ("B4", 0.25),
]


def user_lp():
    while True:
        overall_time = 0
        for note, duration in tune_sequence:
            frame = player.play_tone(note, duration)
            entry = {
                "raw_data": frame,
            }
            ui.send_message("audio_frame", entry)
            overall_time += duration

        time.sleep(overall_time)  # wait for the whole sequence to finish before restarting


App.run(user_loop=user_lp)
