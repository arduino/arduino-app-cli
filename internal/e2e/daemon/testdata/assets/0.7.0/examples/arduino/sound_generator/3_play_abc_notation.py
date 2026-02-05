# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME: Play music in ABC notation
from arduino.app_bricks.sound_generator import SoundGenerator, SoundEffect
from arduino.app_utils import App

player = SoundGenerator(sound_effects=[SoundEffect.adsr()])


def play_melody():
    abc_music = """
    X:1
    T:Twinkle, Twinkle Little Star - #11
    T:Alphabet Song
    C:Traditional Kid's Song
    M:4/4
    L:1/4
    K:D
    |"D"D D A A|"G"B B "D"A2
    |"G"G G "D"F F|"A"E/2E/2E/2E/2 "D"D2
    |A A "G"G G|"D"F F "A"E2
    |"D"A A "G"G G|"D"F F "A"E2
    |"D"D D A A|"G"B B "D"A2
    |"G"G G "D"F F|"A"E E "D"D2|
    """
    player.play_abc(abc_music, wait_completion=True)


App.run(user_loop=play_melody)
